package driver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/nuberabe/ovc-sdk-go/ovc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
)

const (
	//createdByNub is used to tag a description to a disk created by the CSI Driver
	createdByNub = "Created by Nubera CSI Driver"

	// minimumVolumeSizeInBytes is used to validate that the user is not trying
	// to create a volume that is smaller than what we support
	minimumVolumeSizeInBytes int64 = 1 * GB

	// maximumVolumeSizeInBytes is used to validate that the user is not trying
	// to create a volume that is larger than what we support
	maximumVolumeSizeInBytes int64 = 2000 * GB

	// defaultVolumeSizeInBytes is used when the user did not provide a size or
	// the size they provided did not satisfy our requirements
	defaultVolumeSizeInBytes int64 = 10 * GB
)

// CreateVolume creates a new volume from the given request. The function is
// idempotent.
func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}

	if req.VolumeCapabilities == nil || len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}

	size, err := extractStorage(req.CapacityRange)
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "invalid capacity range: %v", err)
	}

	volumeName := req.Name

	// get volume first, if it's created do no thing
	volumes, err := d.client.Disks.List(d.accountID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// volume already exist, do nothing
	for _, vol := range *volumes {
		if vol.Name == req.Name {
			d.log.Info("Volume was already created")
			return &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					VolumeId:      strconv.Itoa(vol.ID),
					CapacityBytes: int64(vol.Size) * GB,
				},
			}, nil
		}
	}

	diskConfig := &ovc.DiskConfig{
		Name:        volumeName,
		Description: createdByNub,
		Size:        int(size / GB),
		AccountID:   d.accountID,
		GID:         d.gid,
		Type:        "D",
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_name":             volumeName,
		"storage_size_giga_bytes": size / GB,
		"method":                  "create_volume",
		"volume_capabilities":     req.VolumeCapabilities,
	})
	ll.Info("create volume called")

	ll.WithField("volume_req", diskConfig).Info("creating volume")
	volID, err := d.client.Disks.Create(diskConfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volID,
			CapacityBytes: size,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						"region": "switzerland",
					},
				},
			},
		},
	}

	ll.WithField("response", resp).Info("volume created")
	return resp, nil
}

// DeleteVolume deletes the given volume. The function is idempotent.
func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id": req.VolumeId,
		"method":    "delete_volume",
	})
	ll.Info("delete volume called")

	volID, err := strconv.Atoi(req.VolumeId)
	if err != nil {
		return nil, err
	}

	deleteConfig := &ovc.DiskDeleteConfig{
		DiskID:      volID,
		Detach:      true,
		Permanently: true,
	}

	err = d.client.Disks.Delete(deleteConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	ll.Info("volume is deleted")
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume attaches the given volume to the node
func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}

	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Node ID must be provided")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume capability must be provided")
	}

	if req.Readonly {
		return nil, status.Error(codes.AlreadyExists, "read only Volumes are not supported")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id": req.VolumeId,
		"node_id":   req.NodeId,
		"method":    "controller_publish_volume",
	})
	ll.Info("controller publish volume called")

	// check if volume exist before trying to attach it
	vol, err := d.client.Disks.Get(req.VolumeId)
	if err != nil {
		return nil, err
	}

	_, err = d.client.Machines.Get(req.NodeId)
	if err != nil {
		return nil, err
	}

	machineID, err := strconv.Atoi(req.NodeId)
	if err != nil {
		return nil, err
	}

	diskID, err := strconv.Atoi(req.VolumeId)
	if err != nil {
		return nil, err
	}

	diskConfig := &ovc.DiskAttachConfig{
		MachineID: machineID,
		DiskID:    diskID,
	}

	// attach the volume to the correct node
	err = d.client.Disks.Attach(diskConfig)
	if err != nil {
		if strings.Contains(err.Error(), "This disk is already attached to another machine") {
			ll.WithFields(logrus.Fields{
				"error": err,
			}).Warn("assuming volume is attached already")
		}
		return nil, err
	}

	disk, err := d.client.Disks.Get(req.VolumeId)
	if err != nil {
		return nil, err
	}

	ll.Info("volume is attached")
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"PublishInfoVolumeName": vol.Name,
			"PublishInfoVolumeID":   strconv.Itoa(vol.ID),
			"PublishInfoNodeID":     req.NodeId,
			"OrderNumber":           strconv.Itoa(disk.Order),
		},
	}, nil
}

// ControllerUnpublishVolume deattaches the given volume from the node
func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}

	machineID, err := strconv.Atoi(req.NodeId)
	if err != nil {
		d.log.WithField("node_id", req.NodeId).Warn("node ID cannot be converted to an integer")
	}

	volID, err := strconv.Atoi(req.VolumeId)
	if err != nil {
		d.log.WithField("volume_id", req.VolumeId).Warn("volume ID cannot be converted to an integer")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id":  req.VolumeId,
		"node_id":    req.NodeId,
		"machine_id": machineID,
		"method":     "controller_unpublish_volume",
	})
	ll.Info("controller unpublish volume called")

	diskConfig := &ovc.DiskAttachConfig{
		MachineID: machineID,
		DiskID:    volID,
	}

	err = d.client.Disks.Detach(diskConfig)
	if err != nil {
		ll.Infof("fail detatching the volume: %q", err)
		return nil, err
	}
	ll.Info("volume is detached")
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities checks whether the volume capabilities requested
// are supported.
func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume ID must be provided")
	}

	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume Capabilities must be provided")
	}

	ll := d.log.WithFields(logrus.Fields{
		"volume_id":              req.VolumeId,
		"volume_capabilities":    req.VolumeCapabilities,
		"supported_capabilities": d.volumeCaps,
		"method":                 "validate_volume_capabilities",
	})
	ll.Info("validate volume capabilities called")

	if _, err := d.client.Disks.Get(req.VolumeId); err != nil {
		return nil, status.Error(codes.NotFound, "Volume not found")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	if d.isValidVolumeCapabilities(volCaps) {
		confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{VolumeCapabilities: volCaps}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: confirmed,
	}, nil
}

// ListVolumes returns a list of all requested volumes
func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	var err error
	ll := d.log.WithFields(logrus.Fields{
		"account_id": d.accountID,
		"method":     "list_volumes",
	})
	ll.Info("list volumes called")

	disks, err := d.client.Disks.List(d.accountID)
	if err != nil {
		return nil, err
	}

	var entries []*csi.ListVolumesResponse_Entry
	for _, disk := range *disks {
		diskID := strconv.Itoa(disk.ID)
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      diskID,
				CapacityBytes: int64(disk.Size),
			},
		})
	}

	resp := &csi.ListVolumesResponse{
		Entries: entries,
	}

	ll.WithField("response", resp).Info("volumes listed")
	return resp, nil
}

// GetCapacity returns the capacity of the storage pool
func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	// TODO: not able to return capacity of the storage pool
	d.log.WithFields(logrus.Fields{
		"params": req.Parameters,
		"method": "get_capacity",
	}).Warn("get capacity is not implemented")
	return nil, status.Error(codes.Unimplemented, "")
}

// Expands the volume. Currently using csi1.0 which doesn't support expanding
// func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
// 	// TODO: no resize support
// 	d.log.WithFields(logrus.Fields{
// 		"volume_id": req.VolumeId,
// 		"method":    "resize_volume",
// 	}).Warn("create snapshot is not implemented")
// 	return nil, status.Error(codes.Unimplemented, "")
// }

// CreateSnapshot creates a snaphot of the volume
// Currently not supported by the OVC API
func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	// TODO: no snapshot support
	d.log.WithFields(logrus.Fields{
		"params": req.Parameters,
		"method": "create_snapshot",
	}).Warn("create snapshot is not implemented")
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot deletes a snaphot
// Currently not supported by the OVC API
func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// TODO: no snapshot support
	d.log.WithFields(logrus.Fields{
		"snapshot_id": req.SnapshotId,
		"method":      "delete_snapshot",
	}).Warn("delete snapshot is not implemented")
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots lists all snaphot
// Currently not supported by the OVC API
func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	// TODO: no snapshot support
	d.log.WithFields(logrus.Fields{
		"snapshot_id": req.SnapshotId,
		"method":      "list_snapshot",
	}).Warn("list snapshot is not implemented")
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities returns the capabilities of the controller service.
func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {

	var caps []*csi.ControllerServiceCapability
	for _, cap := range d.controllerCaps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}

	d.log.WithFields(logrus.Fields{
		"method": "controller_get_capabilities",
	}).Info("controller get capabilities called")
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

// extractStorage extracts the storage size in bytes from the given capacity
// range. If the capacity range is not satisfied it returns the default volume
// size. If the capacity range is below or above supported sizes, it returns an
// error.
func extractStorage(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return defaultVolumeSizeInBytes, nil
	}

	requiredBytes := capRange.GetRequiredBytes()
	requiredSet := 0 < requiredBytes
	limitBytes := capRange.GetLimitBytes()
	limitSet := 0 < limitBytes

	if !requiredSet && !limitSet {
		return defaultVolumeSizeInBytes, nil
	}

	if requiredSet && limitSet && limitBytes < requiredBytes {
		return 0, fmt.Errorf("limit (%v) can not be less than required (%v) size", formatBytes(limitBytes), formatBytes(requiredBytes))
	}

	if requiredSet && !limitSet && requiredBytes < minimumVolumeSizeInBytes {
		return 0, fmt.Errorf("required (%v) can not be less than minimum supported volume size (%v)", formatBytes(requiredBytes), formatBytes(minimumVolumeSizeInBytes))
	}

	if limitSet && limitBytes < minimumVolumeSizeInBytes {
		return 0, fmt.Errorf("limit (%v) can not be less than minimum supported volume size (%v)", formatBytes(limitBytes), formatBytes(minimumVolumeSizeInBytes))
	}

	if requiredSet && requiredBytes > maximumVolumeSizeInBytes {
		return 0, fmt.Errorf("required (%v) can not exceed maximum supported volume size (%v)", formatBytes(requiredBytes), formatBytes(maximumVolumeSizeInBytes))
	}

	if !requiredSet && limitSet && limitBytes > maximumVolumeSizeInBytes {
		return 0, fmt.Errorf("limit (%v) can not exceed maximum supported volume size (%v)", formatBytes(limitBytes), formatBytes(maximumVolumeSizeInBytes))
	}

	if requiredSet && limitSet && requiredBytes == limitBytes {
		return requiredBytes, nil
	}

	if requiredSet {
		return requiredBytes, nil
	}

	if limitSet {
		return limitBytes, nil
	}

	return defaultVolumeSizeInBytes, nil
}

func formatBytes(inputBytes int64) string {
	output := float64(inputBytes)
	unit := ""

	switch {
	case inputBytes >= TB:
		output = output / TB
		unit = "Ti"
	case inputBytes >= GB:
		output = output / GB
		unit = "Gi"
	case inputBytes >= MB:
		output = output / MB
		unit = "Mi"
	case inputBytes >= KB:
		output = output / KB
		unit = "Ki"
	case inputBytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(output, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

func (d *Driver) isValidVolumeCapabilities(volCaps []*csi.VolumeCapability) bool {
	hasSupport := func(cap *csi.VolumeCapability) bool {
		for _, c := range d.volumeCaps {
			if c.GetMode() == cap.AccessMode.GetMode() {
				return true
			}
		}
		return false
	}

	foundAll := true
	for _, c := range volCaps {
		if !hasSupport(c) {
			foundAll = false
		}
	}
	return foundAll
}

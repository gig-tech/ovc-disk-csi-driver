/*
Copyright 2018-2019 GIG TECHNOLOGY NV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/gig-tech/ovc-sdk-go/ovc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/util/mount"
)

// Driver struct contains all relevant Driver information
type Driver struct {
	endpoint  string
	client    *ovc.Client
	accountID int
	gridID    int
	nodeID    string

	volumeCaps     []csi.VolumeCapability_AccessMode
	controllerCaps []csi.ControllerServiceCapability_RPC_Type
	nodeCaps       []csi.NodeServiceCapability_RPC_Type

	srv     *grpc.Server
	log     *logrus.Entry
	mounter *mount.SafeFormatAndMount
}

var (
	version string
)

// NewDriver creates a new driver
func NewDriver(url, endpoint, nodeID string, accountID int, mounter *mount.SafeFormatAndMount, ovcJWT string, verbose bool) (*Driver, error) {
	c := &ovc.Config{
		URL:     url,
		JWT:     ovcJWT,
		Verbose: verbose,
	}
	client, err := ovc.NewClient(c)
	if err != nil {
		return nil, err
	}

	// Fetch grid ID
	locations, err := client.Locations.List()
	if err != nil {
		return nil, err
	}
	gridID := (*locations)[0].GridID

	if mounter == nil {
		mounter = newSafeMounter()
	}

	if nodeID == "" {
		nodeID, err = getNodeID(client)
		if err != nil {
			return nil, fmt.Errorf("something went wrong fetching the node ID %s", err)
		}
	}

	log := logrus.New()
	if verbose {
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}
	logEntry := log.WithFields(logrus.Fields{
		"node_id": nodeID,
	})

	return &Driver{
		gridID:    gridID,
		client:    client,
		endpoint:  endpoint,
		accountID: accountID,
		nodeID:    nodeID,
		mounter:   mounter,
		log:       logEntry,
		volumeCaps: []csi.VolumeCapability_AccessMode{
			{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
		controllerCaps: []csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		},
		nodeCaps: []csi.NodeServiceCapability_RPC_Type{
			csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		},
	}, nil
}

// Run runs the driver
func (d *Driver) Run() error {
	u, err := url.Parse(d.endpoint)
	if err != nil {
		return err
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))

	switch u.Scheme {
	case "unix", "unixgram", "unixpacket":
		// Check if file already exists and clean it up
		if _, err := os.Stat(u.Path); err == nil {
			err := os.Remove(u.Path)
			if err != nil {
				return fmt.Errorf("failed to remove socket file (%s): %s", u.Path, err)
			}
		}
	}

	listener, err := net.Listen(u.Scheme, addr)
	if err != nil {
		return err
	}

	logErr := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			d.log.Errorf("GRPC error: %v", err)
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
	}
	d.srv = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	d.log.Infof("Listening for connections on address: %#v", listener.Addr())

	return d.srv.Serve(listener)
}

// Stop stops the plugin
func (d *Driver) Stop() {
	d.log.Info("server stopped")
	d.srv.Stop()
}

// GetVersion returns the current version
func GetVersion() string {
	return version
}

func newSafeMounter() *mount.SafeFormatAndMount {
	return &mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      mount.NewOsExec(),
	}
}

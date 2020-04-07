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
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/gig-tech/ovc-sdk-go/v3/ovc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/util/mount"
)

type attachConfig struct {
	machineID int
	diskID    int
	result    chan error
}

// Driver struct contains all relevant Driver information
type Driver struct {
	endpoint     string
	client       *ovc.Client
	accountID    int
	gridID       int
	nodeID       string
	cloudspaceID int

	attacher bool
	attach   chan attachConfig
	detach   chan attachConfig

	volumeCaps     []csi.VolumeCapability_AccessMode
	controllerCaps []csi.ControllerServiceCapability_RPC_Type
	nodeCaps       []csi.NodeServiceCapability_RPC_Type

	srv     *grpc.Server
	log     *logrus.Entry
	mounter *mount.SafeFormatAndMount

	quit         chan bool
	jwtRefresher *time.Ticker
}

var (
	version string
)

// NewDriver creates a new driver
func NewDriver(url, endpoint, account string, mounter *mount.SafeFormatAndMount, ovcJWT string, verbose bool, attacher bool) (*Driver, error) {
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

	accountID, err := client.Accounts.GetIDByName(account)
	if err != nil {
		return nil, err
	}

	if mounter == nil {
		mounter = newSafeMounter()
	}

	nodeID, cloudspaceID, err := getNodeID(client)
	if err != nil {
		return nil, fmt.Errorf("something went wrong fetching the node ID %s", err)
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

	driver := &Driver{
		gridID:       gridID,
		client:       client,
		endpoint:     endpoint,
		accountID:    accountID,
		nodeID:       nodeID,
		cloudspaceID: cloudspaceID,
		mounter:      mounter,
		log:          logEntry,
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
		attacher: attacher,
		quit:     make(chan bool),
	}

	driver.log.Info("Starting JWT maintainer to refresh the JWT at least once each 30 days.")
	driver.client.JWT.Get()
	driver.jwtRefresher = time.NewTicker(29 * 24 * time.Hour)
	go func() {
		for {
			select {
			case <-driver.quit:
				return
			case <-driver.jwtRefresher.C:
				for {
					if _, err := driver.client.JWT.Get(); err != nil {
						driver.log.Errorf("Error refreshing the JWT: %s", err)
					} else {
						break
					}
					// Sleep 60 seconds before retrying unless quiting
					for i := 0; i < 60; i++ {
						select {
						case <-driver.quit:
							return
						default:
							time.Sleep(1 * time.Second)
						}
					}
				}
			}
		}
	}()

	if attacher {
		driver.attach = make(chan attachConfig)
		driver.detach = make(chan attachConfig)
		go driver.runOVCStatemachine()
	}

	return driver, nil
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
	d.log.Info("Server stopped")
	d.srv.Stop()
	d.log.Info("Waiting for JWT refresher to finish")
	close(d.quit)
	if d.attacher {
		close(d.attach)
		close(d.detach)
	}
	d.jwtRefresher.Stop()
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

func (d *Driver) createStateInventory() (map[int][]int, error) {
	machines, err := d.client.Machines.List(d.cloudspaceID)
	if err != nil {
		return nil, err
	}
	state := make(map[int][]int)
	for _, machine := range *machines {
		state[machine.ID] = machine.Disks
	}
	return state, nil
}

func (d *Driver) runOVCStatemachine() {
	var state map[int][]int
	var ac attachConfig

	newStateMachine := func() {
		var err error
		d.log.Info("Creating ovc state machine")
		for {
			if state, err = d.createStateInventory(); err == nil {
				break
			}
			d.log.Warning("Failed to create state machine. Retrying in 30 seconds")
			time.Sleep(30 * time.Second)
		}
	}

	indexOf := func(slice []int, value int) int {
		for i, v := range slice {
			if v == value {
				return i
			}
		}
		return -1
	}

	remove := func(slice []int, i int) []int {
		slice[len(slice)-1], slice[i] = slice[i], slice[len(slice)-1]
		return slice[:len(slice)-1]
	}

	attach := func() error {
		for machineID, disks := range state {
			if index := indexOf(disks, ac.diskID); index >= 0 {
				if machineID == ac.machineID {
					d.log.Infof("Nothing to do, this disk %d is already attached to machine %d", ac.diskID, machineID)
					ac.result <- nil
					return nil
				}
				// Disk is attached to the wrong machine: disconnect
				if err := d.client.Disks.Detach(&ovc.DiskAttachConfig{
					MachineID: machineID,
					DiskID:    ac.diskID,
				}); err != nil {
					d.log.Errorf("Failed to detach disk %d from machine %d: %s", ac.diskID, machineID, err)
					ac.result <- err
					return err
				}
				d.log.Infof("Detached disk %d from machine %d", ac.diskID, machineID)
				state[machineID] = remove(disks, index)
				break
			}
		}
		// Attach the disk to the correct machine
		if err := d.client.Disks.Attach(&ovc.DiskAttachConfig{
			MachineID: ac.machineID,
			DiskID:    ac.diskID,
		}); err != nil {
			d.log.Errorf("Failed to attach disk %d to machine %d: %s", ac.diskID, ac.machineID, err)
			ac.result <- err
			return err
		}
		state[ac.machineID] = append(state[ac.machineID], ac.diskID)
		ac.result <- nil
		d.log.Errorf("Attached disk %d to machine %d", ac.diskID, ac.machineID)
		return nil
	}

	detach := func() error {
		for machineID, disks := range state {
			if index := indexOf(disks, ac.diskID); index >= 0 {
				if err := d.client.Disks.Detach(&ovc.DiskAttachConfig{
					MachineID: machineID,
					DiskID:    ac.diskID,
				}); err != nil {
					d.log.Errorf("Failed to detach disk %d from machine %d: %s", ac.diskID, machineID, err)
					ac.result <- err
					return err
				}
				d.log.Infof("Detached disk %d from machine %d", ac.diskID, machineID)
				state[machineID] = remove(disks, index)
				break
			}
		}
		ac.result <- nil
		return nil
	}

	newStateMachine()
	for {
		select {
		case ac = <-d.attach:
			if err := attach(); err != nil {
				d.log.Info("Error while executing attach request. Recycling state machine")
				newStateMachine()
			}
		case ac = <-d.detach:
			if err := detach(); err != nil {
				d.log.Info("Error while executing detach request. Recycling state machine")
				newStateMachine()
			}
		case <-d.quit:
			break
		}
	}
}

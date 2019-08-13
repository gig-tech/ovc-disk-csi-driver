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
	"github.com/gig-tech/ovc-disk-csi-driver/config"
	"github.com/gig-tech/ovc-sdk-go/ovc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/util/mount"
)

// Driver struct contains all relevant Driver information
type Driver struct {
	endpoint string
	nodeID   string
	g8s      map[string]g8
	nodeG8   string // name of g8 this instance is running on

	volumeCaps     []csi.VolumeCapability_AccessMode
	controllerCaps []csi.ControllerServiceCapability_RPC_Type
	nodeCaps       []csi.NodeServiceCapability_RPC_Type

	srv     *grpc.Server
	log     *logrus.Entry
	mounter *mount.SafeFormatAndMount
}

type g8 struct {
	client    *ovc.Client
	accountID int
	gridID    int
}

var (
	version string
)

// NewDriver creates a new driver
func NewDriver(driverCfg *config.Driver, mounter *mount.SafeFormatAndMount) (*Driver, error) {
	log := logrus.New()
	if driverCfg.Verbose {
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}

	g8Configs, err := generateG8s(driverCfg)
	if err != nil {
		return nil, fmt.Errorf("failed generating g8 configs: %s", err)
	}

	if mounter == nil {
		mounter = newSafeMounter()
	}

	nodeG8, err := currentG8(g8Configs, log)
	if err != nil {
		return nil, fmt.Errorf("failed fetching node's g8: %s", err)
	}

	machineID, err := getMachineID(g8Configs[nodeG8].client)
	if err != nil {
		return nil, fmt.Errorf("failed fetching the node ID %s", err)
	}

	// TODO: fetch G8 name
	nodeID := machineID

	logEntry := log.WithFields(logrus.Fields{
		"node_id": nodeID,
	})

	return &Driver{
		endpoint: driverCfg.Endpoint,
		g8s:      g8Configs,
		nodeG8:   nodeG8,
		nodeID:   nodeID,
		mounter:  mounter,
		log:      logEntry,
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

// generateG8s generates g8 clients
func generateG8s(c *config.Driver) (map[string]g8, error) {
	g8s := make(map[string]g8)
	for _, g8Config := range c.G8s {
		clientC := &ovc.Config{
			URL:     g8Config.URL,
			JWT:     g8Config.JWT,
			Verbose: c.Verbose,
		}
		client, err := ovc.NewClient(clientC)
		if err != nil {
			return nil, fmt.Errorf("failed generating client for %s: %s", g8Config.Name, err)
		}

		locations, err := client.Locations.List()
		if err != nil {
			return nil, fmt.Errorf("failed listing locations for %s: %s", g8Config.Name, err)
		}
		gridID := (*locations)[0].GridID

		accountID, err := client.Accounts.GetIDByName(g8Config.Account)
		if err != nil {
			return nil, fmt.Errorf("failed getting account ID for %s: %s", g8Config.Name, err)
		}

		g8 := g8{
			client:    client,
			accountID: accountID,
			gridID:    gridID,
		}

		g8s[g8Config.Name] = g8
	}

	return g8s, nil
}

func currentG8(g8s map[string]g8, log *logrus.Logger) (string, error) {
	nodeUUID := ""

	for g8, g8Config := range g8s {
		log.Debugf("Looking for node on G8 %s", g8)

		nodeUUID, err := getMachineID(g8Config.client)
		if err != nil {
			continue
		}

		_, err = g8Config.client.Machines.GetByReferenceID(nodeUUID)
		if err == nil {
			return g8, nil
		}
	}

	return "", fmt.Errorf("G8 not found for machine %s", nodeUUID)
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

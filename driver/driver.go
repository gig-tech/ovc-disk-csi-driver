package driver

import (
	"context"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/nuberabe/ovc-sdk-go/ovc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/util/mount"
)

// Driver struct contains all relevant Driver information
type Driver struct {
	endpoint  string
	client    *ovc.OvcClient
	accountID int
	gid       int
	nodeid    string

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
func NewDriver(url, ep, nodeid string, accountID, gid int, mounter *mount.SafeFormatAndMount) (*Driver, error) {

	c := &ovc.Config{
		Hostname:     url,
		ClientID:     os.Getenv("OVC_CLIENT_ID"),
		ClientSecret: os.Getenv("OVC_CLIENT_SECRET"),
	}
	client := ovc.NewClient(c, url)

	if mounter == nil {
		mounter = newSafeMounter()
	}

	log := logrus.New().WithFields(logrus.Fields{
		"node_id": nodeid,
	})

	return &Driver{
		gid:       gid,
		client:    client,
		endpoint:  ep,
		accountID: accountID,
		nodeid:    nodeid,
		mounter:   mounter,
		log:       log,
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

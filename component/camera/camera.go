// Package camera defines a frame capturing device.
package camera

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.CameraService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterCameraServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the camera resource subtype string.
const SubtypeName = resource.SubtypeName("camera")

// Subtype is a constant that identifies the camera resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named cameras's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Camera represents anything that can capture frames.
type Camera interface {
	gostream.ImageSource
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
}

// ImageSource implements a Camera with a gostream.ImageSource.
type ImageSource struct {
	gostream.ImageSource
}

// Close closes the underlying ImageSource.
func (is *ImageSource) Close(ctx context.Context) error {
	return viamutils.TryClose(ctx, is.ImageSource)
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported.
func (is *ImageSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if c, ok := is.ImageSource.(Camera); ok {
		return c.NextPointCloud(ctx)
	}
	img, closer, err := is.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer closer()
	return rimage.ConvertToImageWithDepth(img).ToPointCloud()
}

// WrapWithReconfigurable wraps a camera with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	c, ok := r.(Camera)
	if !ok {
		return nil, errors.Errorf("expected resource to be Camera but got %T", r)
	}
	if reconfigurable, ok := c.(*reconfigurableCamera); ok {
		return reconfigurable, nil
	}
	return &reconfigurableCamera{actual: c}, nil
}

var (
	_ = Camera(&reconfigurableCamera{})
	_ = resource.Reconfigurable(&reconfigurableCamera{})
)

// FromRobot is a helper for getting the named Camera from the given Robot.
func FromRobot(r robot.Robot, name string) (Camera, bool) {
	res, ok := r.ResourceByName(Named(name))
	if ok {
		part, ok := res.(Camera)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// NamesFromRobot is a helper for getting all camera names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableCamera struct {
	mu     sync.RWMutex
	actual Camera
}

func (c *reconfigurableCamera) ProxyFor() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual
}

func (c *reconfigurableCamera) Next(ctx context.Context) (image.Image, func(), error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Next(ctx)
}

func (c *reconfigurableCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.NextPointCloud(ctx)
}

func (c *reconfigurableCamera) Close(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return viamutils.TryClose(ctx, c.actual)
}

// Reconfigure reconfigures the resource.
func (c *reconfigurableCamera) Reconfigure(ctx context.Context, newCamera resource.Reconfigurable) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	actual, ok := newCamera.(*reconfigurableCamera)
	if !ok {
		return utils.NewUnexpectedTypeError(c, newCamera)
	}
	if err := viamutils.TryClose(ctx, c.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	c.actual = actual.actual
	return nil
}

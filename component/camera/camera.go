// Package camera defines a frame capturing device.
package camera

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	pb "go.viam.com/rdk/proto/api/component/camera/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
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
		RPCServiceDesc: &pb.CameraService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})

	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: nextPointCloud.String(),
	}, newNextPointCloudCollector)
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
	generic.Generic
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
	GetProperties(ctx context.Context) (rimage.Projector, error)
}

// A PointCloudSource is a source that can generate pointclouds.
type PointCloudSource interface {
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
}

// GetProjector either gets the camera parameters from the config, or if the camera has a parent source,
// can copy over the projector from there. If the camera doesn't have a projector, will return false.
func GetProjector(ctx context.Context, attrs *AttrConfig, parentSource Camera) (rimage.Projector, bool) {
	// if the camera parameters are specified in the config, those get priority.
	if attrs != nil && attrs.CameraParameters != nil {
		return attrs.CameraParameters, true
	}
	// inherit camera parameters from source camera if possible.
	if parentSource != nil {
		proj, err := parentSource.GetProperties(ctx)
		if errors.Is(err, transform.ErrNoIntrinsics) {
			return nil, false
		} else if err != nil {
			panic(err)
		}
		return proj, true
	}
	return nil, false
}

// New creates a Camera either with or without a projector.
func New(imgSrc gostream.ImageSource, proj rimage.Projector) (Camera, error) {
	if imgSrc == nil {
		return nil, errors.New("cannot have a nil image source")
	}
	return &imageSource{imgSrc, proj, generic.Unimplemented{}}, nil
}

// ImageSource implements a Camera with a gostream.ImageSource.
type imageSource struct {
	gostream.ImageSource
	projector rimage.Projector
	generic.Unimplemented
}

// Close closes the underlying ImageSource.
func (is *imageSource) Close(ctx context.Context) error {
	return viamutils.TryClose(ctx, is.ImageSource)
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported.
func (is *imageSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::imageSource::NextPointCloud")
	defer span.End()
	if c, ok := is.ImageSource.(PointCloudSource); ok {
		return c.NextPointCloud(ctx)
	}
	if is.projector == nil {
		return nil, transform.NewNoIntrinsicsError("cannot do a projection to a point cloud")
	}
	img, release, err := is.Next(ctx)
	defer release()
	if err != nil {
		return nil, err
	}
	dm, ok := img.(*rimage.DepthMap)
	if !ok {
		return nil, errors.New("image has no depth information to project to pointcloud")
	}
	return dm.ToPointCloud(is.projector), nil
}

func (is *imageSource) GetProperties(ctx context.Context) (rimage.Projector, error) {
	if is.projector != nil {
		return is.projector, nil
	}
	return nil, transform.NewNoIntrinsicsError("No features in config")
}

// WrapWithReconfigurable wraps a camera with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	c, ok := r.(Camera)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Camera", r)
	}
	if reconfigurable, ok := c.(*reconfigurableCamera); ok {
		return reconfigurable, nil
	}
	return &reconfigurableCamera{actual: c}, nil
}

var (
	_ = Camera(&reconfigurableCamera{})
	_ = resource.Reconfigurable(&reconfigurableCamera{})
	_ = viamutils.ContextCloser(&reconfigurableCamera{})
)

// FromDependencies is a helper for getting the named camera from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (Camera, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(Camera)
	if !ok {
		return nil, utils.DependencyTypeError(name, "Camera", res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named Camera from the given Robot.
func FromRobot(r robot.Robot, name string) (Camera, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Camera)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Camera", res)
	}
	return part, nil
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

func (c *reconfigurableCamera) GetProperties(ctx context.Context) (rimage.Projector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.GetProperties(ctx)
}

func (c *reconfigurableCamera) Close(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return viamutils.TryClose(ctx, c.actual)
}

func (c *reconfigurableCamera) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Do(ctx, cmd)
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

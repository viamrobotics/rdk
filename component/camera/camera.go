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

// WithProjector is a camera with the capability to project images to 3D.
type WithProjector interface {
	Camera
	GetProjector() rimage.Projector
}

// Projector will return the camera's projector if it has it, or returns nil if not.
func Projector(cam Camera) rimage.Projector {
	var proj rimage.Projector
	if c, ok := cam.(WithProjector); ok {
		proj = c.GetProjector()
	} else if c, ok := utils.UnwrapProxy(cam).(WithProjector); ok {
		proj = c.GetProjector()
	}
	return proj
}

// New creates a Camera either with or without a projector, depending on if the camera config has the parameters,
// or if it has a parent Camera with camera parameters that it should copy. parentSource and attrs can be nil.
func New(imgSrc gostream.ImageSource, attrs *AttrConfig, parentSource Camera) (Camera, error) {
	if imgSrc == nil {
		return nil, errors.New("cannot have a nil image source")
	}
	// if the camera parameters are specified in the config, those get priority.
	if attrs != nil && attrs.CameraParameters != nil {
		return &imageSourceWithProjector{imgSrc, attrs.CameraParameters, generic.Unimplemented{}}, nil
	}
	// inherit camera parameters from source camera if possible. if not, create a camera without projector.
	if reconfigCam, ok := parentSource.(*reconfigurableCamera); ok {
		if c, ok := reconfigCam.ProxyFor().(WithProjector); ok {
			return &imageSourceWithProjector{imgSrc, c.GetProjector(), generic.Unimplemented{}}, nil
		}
	}
	if camera, ok := parentSource.(WithProjector); ok {
		return &imageSourceWithProjector{imgSrc, camera.GetProjector(), generic.Unimplemented{}}, nil
	}
	return &imageSource{imgSrc, generic.Unimplemented{}}, nil
}

// ImageSource implements a Camera with a gostream.ImageSource.
type imageSource struct {
	gostream.ImageSource
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
	return nil, errors.New("source has no Projector/Camera Intrinsics associated with it to do a projection to a point cloud")
}

func (is *imageSource) GetProperties(ctx context.Context) (rimage.Projector, error) {
	return nil, errors.New("source has no Projector/Camera Intrinsics associated with it")
}

// ImageSourceWithProjector implements a CameraWithProjector with a gostream.ImageSource and Projector.
type imageSourceWithProjector struct {
	gostream.ImageSource
	projector rimage.Projector
	generic.Unimplemented
}

// Close closes the underlying ImageSource.
func (iswp *imageSourceWithProjector) Close(ctx context.Context) error {
	return viamutils.TryClose(ctx, iswp.ImageSource)
}

// Projector returns the camera's Projector.
func (iswp *imageSourceWithProjector) GetProjector() rimage.Projector {
	return iswp.projector
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported.
func (iswp *imageSourceWithProjector) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::imageSourceWithProjector::NextPointCloud")
	defer span.End()
	if c, ok := iswp.ImageSource.(PointCloudSource); ok {
		return c.NextPointCloud(ctx)
	}
	img, closer, err := iswp.Next(ctx)
	if err != nil {
		return nil, err
	}

	dm, ok := img.(*rimage.DepthMap)
	if ok {
		return dm.ToPointCloud(iswp.projector), nil
	}

	defer closer()

	_, toImageWithDepthSpan := trace.StartSpan(ctx, "camera::imageSourceWithProjector::NextPointCloud::ConvertToImageWithDepth")
	imageWithDepth := rimage.ConvertToImageWithDepth(img)
	toImageWithDepthSpan.End()

	_, toPcdSpan := trace.StartSpan(ctx, "camera::imageSourceWithProjector::NextPointCloud::ImageWithDepthToPointCloud")
	defer toPcdSpan.End()
	return iswp.projector.RGBDToPointCloud(imageWithDepth.Color, imageWithDepth.Depth)
}

func (iswp *imageSourceWithProjector) GetProperties(ctx context.Context) (rimage.Projector, error) {
	return iswp.projector, nil
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

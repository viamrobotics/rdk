// Package camera defines a frame capturing device.
package camera

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
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
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: next.String(),
	}, newNextCollector)
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
	generic.Generic
	projectorProvider

	// Stream returns a stream that makes a best effort to return consecutive images
	// that may have a MIME type hint dictated in the context via gostream.WithMIMETypeHint.
	Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error)

	// NextPointCloud returns the next immediately available point cloud, not necessarily one
	// a part of a sequence. In the future, there could be streaming of point clouds.
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
	Close(ctx context.Context) error
}

// ReadImage reads an image from the given source that is immediately available.
func ReadImage(ctx context.Context, src gostream.VideoSource) (image.Image, func(), error) {
	return gostream.ReadImage(ctx, src)
}

type projectorProvider interface {
	Projector(ctx context.Context) (rimage.Projector, error)
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
		proj, err := parentSource.Projector(ctx)
		if errors.Is(err, transform.ErrNoIntrinsics) {
			return nil, false
		} else if err != nil {
			panic(err)
		}
		return proj, true
	}
	return nil, false
}

// NewFromReader creates a Camera either with or without a projector.
func NewFromReader(reader gostream.VideoReader, proj rimage.Projector) (Camera, error) {
	if reader == nil {
		return nil, errors.New("cannot have a nil reader")
	}
	vs := gostream.NewVideoSource(reader, prop.Video{})
	var projectorFunc func(ctx context.Context) (rimage.Projector, error)
	if proj == nil {
		projectorFunc = detectProjectorFunc(reader)
	} else {
		projectorFunc = func(_ context.Context) (rimage.Projector, error) {
			return proj, nil
		}
	}
	return &videoSource{
		projectorFunc: projectorFunc,
		videoSource:   vs,
		videoStream:   gostream.NewEmbeddedVideoStream(vs),
		actualSource:  reader,
	}, nil
}

// NewFromSource creates a Camera either with or without a projector.
func NewFromSource(source gostream.VideoSource, proj rimage.Projector) (Camera, error) {
	if source == nil {
		return nil, errors.New("cannot have a nil source")
	}
	var projectorFunc func(ctx context.Context) (rimage.Projector, error)
	if proj == nil {
		projectorFunc = detectProjectorFunc(source)
	} else {
		projectorFunc = func(_ context.Context) (rimage.Projector, error) {
			return proj, nil
		}
	}
	return &videoSource{
		projectorFunc: projectorFunc,
		videoSource:   source,
		videoStream:   gostream.NewEmbeddedVideoStream(source),
		actualSource:  source,
	}, nil
}

func detectProjectorFunc(from interface{}) func(ctx context.Context) (rimage.Projector, error) {
	projProv, ok := from.(projectorProvider)
	if !ok {
		return nil
	}
	var getPropsOnce sync.Once
	var props rimage.Projector
	var propsErr error
	return func(ctx context.Context) (rimage.Projector, error) {
		getPropsOnce.Do(func() {
			props, propsErr = projProv.Projector(ctx)
		})
		return props, propsErr
	}
}

// videoSource implements a Camera with a gostream.VideoSource.
type videoSource struct {
	videoSource   gostream.VideoSource
	videoStream   gostream.VideoStream
	actualSource  interface{}
	projectorFunc func(ctx context.Context) (rimage.Projector, error)
}

func (vs *videoSource) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	return vs.videoSource.Stream(ctx, errHandlers...)
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported.
func (vs *videoSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::videoSource::NextPointCloud")
	defer span.End()
	if c, ok := vs.actualSource.(PointCloudSource); ok {
		return c.NextPointCloud(ctx)
	}
	if vs.projectorFunc == nil {
		return nil, transform.NewNoIntrinsicsError("cannot do a projection to a point cloud")
	}
	proj, err := vs.projectorFunc(ctx)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, errors.New("cannot have a nil projector")
	}
	img, release, err := vs.videoStream.Next(ctx)
	defer release()
	if err != nil {
		return nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(img)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot project to a point cloud")
	}
	return dm.ToPointCloud(proj), nil
}

func (vs *videoSource) Projector(ctx context.Context) (rimage.Projector, error) {
	if vs.projectorFunc != nil {
		return vs.projectorFunc(ctx)
	}
	return nil, transform.NewNoIntrinsicsError("No features in config")
}

func (vs *videoSource) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if doer, ok := vs.videoSource.(generic.Generic); ok {
		return doer.Do(ctx, cmd)
	}
	return nil, generic.ErrUnimplemented
}

func (vs *videoSource) Close(ctx context.Context) error {
	return multierr.Combine(vs.videoStream.Close(ctx), vs.videoSource.Close(ctx))
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
	cancelCtx, cancel := context.WithCancel(context.Background())
	return &reconfigurableCamera{
		actual:    c,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}, nil
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
	mu        sync.RWMutex
	actual    Camera
	cancelCtx context.Context
	cancel    func()
}

func (c *reconfigurableCamera) ProxyFor() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual
}

func (c *reconfigurableCamera) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.VideoStream, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stream := &reconfigurableCameraStream{
		c:           c,
		errHandlers: errHandlers,
		cancelCtx:   c.cancelCtx,
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if err := stream.init(ctx); err != nil {
		return nil, err
	}

	return stream, nil
}

type reconfigurableCameraStream struct {
	mu          sync.Mutex
	c           *reconfigurableCamera
	errHandlers []gostream.ErrorHandler
	stream      gostream.VideoStream
	cancelCtx   context.Context
}

func (cs *reconfigurableCameraStream) init(ctx context.Context) error {
	var err error
	cs.stream, err = cs.c.actual.Stream(ctx, cs.errHandlers...)
	return err
}

func (cs *reconfigurableCameraStream) Next(ctx context.Context) (image.Image, func(), error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.stream == nil || cs.cancelCtx.Err() != nil {
		if err := func() error {
			cs.c.mu.Lock()
			defer cs.c.mu.Unlock()
			return cs.init(ctx)
		}(); err != nil {
			return nil, nil, err
		}
	}
	return cs.stream.Next(ctx)
}

func (cs *reconfigurableCameraStream) Close(ctx context.Context) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.stream == nil {
		return nil
	}
	return cs.stream.Close(ctx)
}

func (c *reconfigurableCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.NextPointCloud(ctx)
}

func (c *reconfigurableCamera) Projector(ctx context.Context) (rimage.Projector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Projector(ctx)
}

func (c *reconfigurableCamera) Close(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Close(ctx)
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
	c.cancel()
	// reset
	c.actual = actual.actual
	c.cancelCtx = actual.cancelCtx
	c.cancel = actual.cancel
	return nil
}

// UpdateAction helps hint the reconfiguration process on what strategy to use given a modified config.
// See config.ShouldUpdateAction for more information.
func (c *reconfigurableCamera) UpdateAction(conf *config.Component) config.UpdateActionType {
	obj, canUpdate := c.actual.(config.ComponentUpdate)
	if canUpdate {
		return obj.UpdateAction(conf)
	}
	return config.Reconfigure
}

// SimultaneousColorDepthNext will call Next on both the color and depth camera as simultaneously as possible.
func SimultaneousColorDepthNext(ctx context.Context, color, depth gostream.VideoStream) (image.Image, *rimage.DepthMap) {
	wg := sync.WaitGroup{}
	var col image.Image
	var dm *rimage.DepthMap
	// do a parallel request for the color and depth image
	// get color image
	wg.Add(1)
	viamutils.PanicCapturingGo(func() {
		defer wg.Done()
		var err error
		col, _, err = color.Next(ctx)
		if err != nil {
			panic(err)
		}
	})
	// get depth image
	wg.Add(1)
	viamutils.PanicCapturingGo(func() {
		defer wg.Done()
		d, _, err := depth.Next(ctx)
		if err != nil {
			panic(err)
		}
		dm, err = rimage.ConvertImageToDepthMap(d)
		if err != nil {
			panic(err)
		}
	})
	wg.Wait()
	return col, dm
}

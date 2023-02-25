// Package camera defines an image capturing device.
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
	pb "go.viam.com/api/component/camera/v1"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
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
		Subtype:    Subtype,
		MethodName: nextPointCloud.String(),
	}, newNextPointCloudCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: readImage.String(),
	}, newReadImageCollector)
}

// SubtypeName is a constant that identifies the camera resource subtype string.
const SubtypeName = resource.SubtypeName("camera")

// Subtype is a constant that identifies the camera resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named camera's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// Properties is a lookup for a camera's features and settings.
type Properties struct {
	// SupportsPCD indicates that the Camera supports a valid
	// implementation of NextPointCloud
	SupportsPCD      bool
	ImageType        ImageType
	IntrinsicParams  *transform.PinholeCameraIntrinsics
	DistortionParams transform.Distorter
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
	// Properties returns properties that are intrinsic to the particular
	// implementation of a camera
	Properties(ctx context.Context) (Properties, error)
	Close(ctx context.Context) error
}

// ReadImage reads an image from the given source that is immediately available.
func ReadImage(ctx context.Context, src gostream.VideoSource) (image.Image, func(), error) {
	return gostream.ReadImage(ctx, src)
}

type projectorProvider interface {
	Projector(ctx context.Context) (transform.Projector, error)
}

// A PointCloudSource is a source that can generate pointclouds.
type PointCloudSource interface {
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
}

// NewFromReader creates a Camera either with or without a projector. The stream type
// argument is for detecting whether or not the resulting camera supports return
// of pointcloud data in the absence of an implemented NextPointCloud function.
// If this is unknown or not applicable, a value of camera.Unspecified stream can be supplied.
func NewFromReader(
	ctx context.Context,
	reader gostream.VideoReader,
	syst *transform.PinholeCameraModel, imageType ImageType,
) (Camera, error) {
	if reader == nil {
		return nil, errors.New("cannot have a nil reader")
	}
	vs := gostream.NewVideoSource(reader, prop.Video{})
	actualSystem := syst
	if actualSystem == nil {
		srcCam, ok := reader.(Camera)
		if ok {
			props, err := srcCam.Properties(ctx)
			if err != nil {
				return nil, NewPropertiesError("source camera")
			}

			var cameraModel transform.PinholeCameraModel
			cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

			if props.DistortionParams != nil {
				cameraModel.Distortion = props.DistortionParams
			}
			actualSystem = &cameraModel
		}
	}
	return &videoSource{
		system:       actualSystem,
		videoSource:  vs,
		videoStream:  gostream.NewEmbeddedVideoStream(vs),
		actualSource: reader,
		imageType:    imageType,
	}, nil
}

// NewPinholeModelWithBrownConradyDistortion creates a transform.PinholeCameraModel from
// a *transform.PinholeCameraIntrinsics and a *transform.BrownConrady.
// If *transform.BrownConrady is `nil`, transform.PinholeCameraModel.Distortion
// is not set & remains nil, to prevent https://go.dev/doc/faq#nil_error.
func NewPinholeModelWithBrownConradyDistortion(pinholeCameraIntrinsics *transform.PinholeCameraIntrinsics,
	distortion *transform.BrownConrady,
) transform.PinholeCameraModel {
	var cameraModel transform.PinholeCameraModel
	cameraModel.PinholeCameraIntrinsics = pinholeCameraIntrinsics

	if distortion != nil {
		cameraModel.Distortion = distortion
	}
	return cameraModel
}

// NewPropertiesError returns an error specific to a failure in Properties.
func NewPropertiesError(cameraIdentifier string) error {
	return errors.Errorf("failed to get properties from %s", cameraIdentifier)
}

// NewFromSource creates a Camera either with or without a projector. The stream type
// argument is for detecting whether or not the resulting camera supports return
// of pointcloud data in the absence of an implemented NextPointCloud function.
// If this is unknown or not applicable, a value of camera.Unspecified stream can be supplied.
func NewFromSource(
	ctx context.Context,
	source gostream.VideoSource,
	syst *transform.PinholeCameraModel, imageType ImageType,
) (Camera, error) {
	if source == nil {
		return nil, errors.New("cannot have a nil source")
	}
	actualSystem := syst
	if actualSystem == nil {
		srcCam, ok := source.(Camera)
		if ok {
			props, err := srcCam.Properties(ctx)
			if err != nil {
				return nil, NewPropertiesError("source camera")
			}
			var cameraModel transform.PinholeCameraModel
			cameraModel.PinholeCameraIntrinsics = props.IntrinsicParams

			if props.DistortionParams != nil {
				cameraModel.Distortion = props.DistortionParams
			}

			actualSystem = &cameraModel
		}
	}
	return &videoSource{
		system:       actualSystem,
		videoSource:  source,
		videoStream:  gostream.NewEmbeddedVideoStream(source),
		actualSource: source,
		imageType:    imageType,
	}, nil
}

// videoSource implements a Camera with a gostream.VideoSource.
type videoSource struct {
	videoSource  gostream.VideoSource
	videoStream  gostream.VideoStream
	actualSource interface{}
	system       *transform.PinholeCameraModel
	imageType    ImageType
}

// SourceFromCamera returns a gostream.VideoSource from a camera.Camera if possible, else nil.
func SourceFromCamera(cam Camera) (gostream.VideoSource, error) {
	if asSrc, ok := cam.(*videoSource); ok {
		return asSrc.videoSource, nil
	}
	return nil, errors.Errorf("invalid conversion from %T to %v", cam, "*camera.videoSource")
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
	if vs.system == nil || vs.system.PinholeCameraIntrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("cannot do a projection to a point cloud")
	}
	img, release, err := vs.videoStream.Next(ctx)
	defer release()
	if err != nil {
		return nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(ctx, img)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot project to a point cloud")
	}
	return depthadapter.ToPointCloud(dm, vs.system.PinholeCameraIntrinsics), nil
}

func (vs *videoSource) Projector(ctx context.Context) (transform.Projector, error) {
	if vs.system == nil || vs.system.PinholeCameraIntrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("No features in config")
	}
	return vs.system.PinholeCameraIntrinsics, nil
}

func (vs *videoSource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if doer, ok := vs.videoSource.(generic.Generic); ok {
		return doer.DoCommand(ctx, cmd)
	}
	return nil, generic.ErrUnimplemented
}

func (vs *videoSource) Properties(ctx context.Context) (Properties, error) {
	_, supportsPCD := vs.actualSource.(PointCloudSource)
	result := Properties{
		SupportsPCD: supportsPCD,
	}
	if vs.system == nil {
		return result, nil
	}
	if (vs.system.PinholeCameraIntrinsics != nil) && (vs.imageType == DepthStream) {
		result.SupportsPCD = true
	}
	result.ImageType = vs.imageType
	result.IntrinsicParams = vs.system.PinholeCameraIntrinsics

	if vs.system.Distortion != nil {
		result.DistortionParams = vs.system.Distortion
	}

	return result, nil
}

func (vs *videoSource) Close(ctx context.Context) error {
	return multierr.Combine(vs.videoStream.Close(ctx), vs.videoSource.Close(ctx))
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((*Camera)(nil), actual)
}

// DependencyTypeError is used when a resource doesn't implement the expected interface.
func DependencyTypeError(name string, actual interface{}) error {
	return utils.DependencyTypeError(name, (*Camera)(nil), actual)
}

// WrapWithReconfigurable wraps a camera with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	c, ok := r.(Camera)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := c.(*reconfigurableCamera); ok {
		return reconfigurable, nil
	}
	reconfigurable := newReconfigurable(c, name)

	if mon, ok := c.(LivenessMonitor); ok {
		mon.Monitor(func() {
			reconfigurable.mu.Lock()
			defer reconfigurable.mu.Unlock()
			reconfigurable.reconfigureKnownCamera(newReconfigurable(c, name))
		})
	}

	return reconfigurable, nil
}

func newReconfigurable(c Camera, name resource.Name) *reconfigurableCamera {
	cancelCtx, cancel := context.WithCancel(context.Background())
	return &reconfigurableCamera{
		name:      name,
		actual:    c,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}
}

// A LivenessMonitor is responsible for monitoring the liveness of a camera. An example
// is connectivity. Since the model itself knows best about how to maintain this state,
// the reconfigurable offers a safe way to notify if a state needs to be reset due
// to some exceptional event (like a reconnect).
// It is expected that the monitoring code is tied to the lifetime of the resource
// and once the resource is closed, so should the monitor. That is, it should
// no longer send any resets once a Close on its associated resource has returned.
type LivenessMonitor interface {
	Monitor(notifyReset func())
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
		return nil, DependencyTypeError(name, res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named Camera from the given Robot.
func FromRobot(r robot.Robot, name string) (Camera, error) {
	return robot.ResourceFromRobot[Camera](r, Named(name))
}

// NamesFromRobot is a helper for getting all camera names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type reconfigurableCamera struct {
	mu        sync.RWMutex
	name      resource.Name
	actual    Camera
	cancelCtx context.Context
	cancel    func()
}

func (c *reconfigurableCamera) Name() resource.Name {
	return c.name
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

func (c *reconfigurableCamera) Projector(ctx context.Context) (transform.Projector, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Projector(ctx)
}

func (c *reconfigurableCamera) Properties(ctx context.Context) (Properties, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Properties(ctx)
}

func (c *reconfigurableCamera) Close(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Close(ctx)
}

func (c *reconfigurableCamera) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.DoCommand(ctx, cmd)
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
		golog.Global().Errorw("error closing old", "error", err)
	}
	c.reconfigureKnownCamera(actual)
	return nil
}

// assumes lock is held.
func (c *reconfigurableCamera) reconfigureKnownCamera(newCamera *reconfigurableCamera) {
	c.cancel()
	// reset
	c.actual = newCamera.actual
	c.cancelCtx = newCamera.cancelCtx
	c.cancel = newCamera.cancel
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
		dm, err = rimage.ConvertImageToDepthMap(ctx, d)
		if err != nil {
			panic(err)
		}
	})
	wg.Wait()
	return col, dm
}

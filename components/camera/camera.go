// Package camera defines an image capturing device.
package camera

import (
	"context"
	"image"
	"sync"

	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	pb "go.viam.com/api/component/camera/v1"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype[Camera]{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeColl resource.SubtypeCollection[Camera]) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.CameraService_ServiceDesc,
				NewServer(subtypeColl),
				pb.RegisterCameraServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.CameraService_ServiceDesc,
		RPCClient:      NewClientFromConn,
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

// A Camera is a resource that can capture frames.
type Camera interface {
	resource.Resource
	VideoSource
}

// A VideoSource represents anything that can capture frames.
type VideoSource interface {
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

// FromVideoSource creates a Camera resource from a VideoSource.
// Note: this strips away Reconfiguration and DoCommand abilities.
// If needed, implement the Camera another way. For example, a webcam
// implements a Camera manually so that it can atomically reconfigure itself.
func FromVideoSource(name resource.Name, src VideoSource) Camera {
	return &sourceBasedCamera{
		Named:       name.AsNamed(),
		VideoSource: src,
	}
}

type sourceBasedCamera struct {
	resource.Named
	resource.AlwaysRebuild
	VideoSource
}

// NewVideoSourceFromReader creates a VideoSource either with or without a projector. The stream type
// argument is for detecting whether or not the resulting camera supports return
// of pointcloud data in the absence of an implemented NextPointCloud function.
// If this is unknown or not applicable, a value of camera.Unspecified stream can be supplied.
func NewVideoSourceFromReader(
	ctx context.Context,
	reader gostream.VideoReader,
	syst *transform.PinholeCameraModel, imageType ImageType,
) (VideoSource, error) {
	if reader == nil {
		return nil, errors.New("cannot have a nil reader")
	}
	vs := gostream.NewVideoSource(reader, prop.Video{})
	actualSystem := syst
	if actualSystem == nil {
		srcCam, ok := reader.(VideoSource)
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

// WrapVideoSourceWithProjector creates a Camera either with or without a projector. The stream type
// argument is for detecting whether or not the resulting camera supports return
// of pointcloud data in the absence of an implemented NextPointCloud function.
// If this is unknown or not applicable, a value of camera.Unspecified stream can be supplied.
func WrapVideoSourceWithProjector(
	ctx context.Context,
	source gostream.VideoSource,
	syst *transform.PinholeCameraModel, imageType ImageType,
) (VideoSource, error) {
	if source == nil {
		return nil, errors.New("cannot have a nil source")
	}
	actualSystem := syst
	if actualSystem == nil {
		//nolint:staticcheck
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
	if res, ok := vs.videoSource.(resource.Resource); ok {
		return res.DoCommand(ctx, cmd)
	}
	return nil, resource.ErrDoUnimplemented
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

// FromDependencies is a helper for getting the named camera from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Camera, error) {
	return resource.FromDependencies[Camera](deps, Named(name))
}

// FromRobot is a helper for getting the named Camera from the given Robot.
func FromRobot(r robot.Robot, name string) (Camera, error) {
	return robot.ResourceFromRobot[Camera](r, Named(name))
}

// NamesFromRobot is a helper for getting all camera names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// SimultaneousColorDepthNext will call Next on both the color and depth camera as simultaneously as possible.
func SimultaneousColorDepthNext(ctx context.Context, color, depth gostream.VideoStream) (image.Image, *rimage.DepthMap) {
	var wg sync.WaitGroup
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

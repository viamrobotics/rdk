// Package camera defines an image capturing device.
// For more information, see the [camera component docs].
//
// [camera component docs]: https://docs.viam.com/components/camera/
package camera

import (
	"context"
	"image"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/camera/v1"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Camera]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterCameraServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.CameraService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})

	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: nextPointCloud.String(),
	}, newNextPointCloudCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: readImage.String(),
	}, newReadImageCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: getImages.String(),
	}, newGetImagesCollector)
}

// SubtypeName is a constant that identifies the camera resource subtype string.
const SubtypeName = "camera"

// API is a variable that identifies the camera resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named camera's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Properties is a lookup for a camera's features and settings.
type Properties struct {
	// SupportsPCD indicates that the Camera supports a valid
	// implementation of NextPointCloud
	SupportsPCD      bool
	ImageType        ImageType
	IntrinsicParams  *transform.PinholeCameraIntrinsics
	DistortionParams transform.Distorter
	MimeTypes        []string
}

// NamedImage is a struct that associates the source from where the image came from to the Image.
type NamedImage struct {
	Image      image.Image
	SourceName string
}

// A Camera is a resource that can capture frames.
type Camera interface {
	resource.Resource
	VideoSource
}

// A VideoSource represents anything that can capture frames.
// For more information, see the [camera component docs].
//
// Images example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//
//	images, metadata, err := myCamera.Images(context.Background())
//
// Stream example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//
//	// gets the stream from a camera
//	stream, err := myCamera.Stream(context.Background())
//
//	// gets an image from the camera stream
//	img, release, err := stream.Next(context.Background())
//	defer release()
//
// NextPointCloud example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//
//	// gets the properties from a camera
//	properties, err := myCamera.Properties(context.Background())
//
// Close example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//
//	err = myCamera.Close(ctx)
//
// [camera component docs]: https://docs.viam.com/components/camera/
type VideoSource interface {
	// Images is used for getting simultaneous images from different imagers,
	// along with associated metadata (just timestamp for now). It's not for getting a time series of images from the same imager.
	Images(ctx context.Context) ([]NamedImage, resource.ResponseMetadata, error)

	// Stream returns a stream that makes a best effort to return consecutive images
	// that may have a MIME type hint dictated in the context via gostream.WithMIMETypeHint.
	Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error)

	// NextPointCloud returns the next immediately available point cloud, not necessarily one
	// a part of a sequence. In the future, there could be streaming of point clouds.
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)

	// Properties returns properties that are intrinsic to the particular
	// implementation of a camera.
	Properties(ctx context.Context) (Properties, error)

	// Close shuts down the resource and prevents further use.
	Close(ctx context.Context) error
}

// ReadImage reads an image from the given source that is immediately available.
func ReadImage(ctx context.Context, src gostream.VideoSource) (image.Image, func(), error) {
	return gostream.ReadImage(ctx, src)
}

// A PointCloudSource is a source that can generate pointclouds.
type PointCloudSource interface {
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
}

// A ImagesSource is a source that can return a list of images with timestamp.
type ImagesSource interface {
	Images(ctx context.Context) ([]NamedImage, resource.ResponseMetadata, error)
}

type sourceBasedCamera struct {
	resource.Named
	resource.AlwaysRebuild
	VideoSource
	rtpPassthroughSource rtppassthrough.Source
	logging.Logger
}

func (vs *sourceBasedCamera) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.SubscribeRTP(ctx, bufferSize, packetsCB)
	}
	return rtppassthrough.NilSubscription, errors.New("SubscribeRTP unimplemented")
}

func (vs *sourceBasedCamera) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.Unsubscribe(ctx, id)
	}
	return errors.New("Unsubscribe unimplemented")
}

func (vs *videoSource) SubscribeRTP(
	ctx context.Context,
	bufferSize int,
	packetsCB rtppassthrough.PacketCallback,
) (rtppassthrough.Subscription, error) {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.SubscribeRTP(ctx, bufferSize, packetsCB)
	}
	return rtppassthrough.NilSubscription, errors.New("SubscribeRTP unimplemented")
}

func (vs *videoSource) Unsubscribe(ctx context.Context, id rtppassthrough.SubscriptionID) error {
	if vs.rtpPassthroughSource != nil {
		return vs.rtpPassthroughSource.Unsubscribe(ctx, id)
	}
	return errors.New("Unsubscribe unimplemented")
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
	return robot.NamesByAPI(r, API)
}

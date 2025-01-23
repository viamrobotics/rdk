// Package camera defines an image capturing device.
// For more information, see the [camera component docs].
//
// [camera component docs]: https://docs.viam.com/components/camera/
package camera

import (
	"context"
	"fmt"
	"image"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/camera/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
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
	FrameRate        float32
}

// NamedImage is a struct that associates the source from where the image came from to the Image.
type NamedImage struct {
	Image      image.Image
	SourceName string
}

// ImageMetadata contains useful information about returned image bytes such as its mimetype.
type ImageMetadata struct {
	MimeType string
}

// A Camera is a resource that can capture frames.
type Camera interface {
	resource.Resource
	VideoSource
}

// VideoSource represents anything that can capture frames.
// For more information, see the [camera component docs].
//
// Image example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//	imageBytes, mimeType, err := myCamera.Image(context.Background(), utils.MimeTypeJPEG, nil)
//
// Or try to directly decode as an image.Image:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//	img, err = camera.DecodeImageFromCamera(context.Background(), utils.MimeTypeJPEG, nil, myCamera)
//
// For more information, see the [Image method docs].
//
// Images example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//
//	images, metadata, err := myCamera.Images(context.Background())
//
// For more information, see the [Images method docs].
//
// NextPointCloud example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//
//	// gets the next point cloud from a camera
//	pointCloud, err := myCamera.NextPointCloud(context.Background())
//
// For more information, see the [NextPointCloud method docs].
//
// Close example:
//
//	myCamera, err := camera.FromRobot(machine, "my_camera")
//
//	err = myCamera.Close(context.Background())
//
// For more information, see the [Close method docs].
//
// [camera component docs]: https://docs.viam.com/dev/reference/apis/components/camera/
// [Image method docs]: https://docs.viam.com/dev/reference/apis/components/camera/#getimage
// [Images method docs]: https://docs.viam.com/dev/reference/apis/components/camera/#getimages
// [NextPointCloud method docs]: https://docs.viam.com/dev/reference/apis/components/camera/#getpointcloud
// [Close method docs]: https://docs.viam.com/dev/reference/apis/components/camera/#close
type VideoSource interface {
	// Image returns a byte slice representing an image that tries to adhere to the MIME type hint.
	// Image also may return metadata about the frame.
	Image(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, ImageMetadata, error)

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

// DecodeImageFromCamera retrieves image bytes from a camera resource and serializes it as an image.Image.
func DecodeImageFromCamera(ctx context.Context, mimeType string, extra map[string]interface{}, cam Camera) (image.Image, error) {
	resBytes, resMetadata, err := cam.Image(ctx, mimeType, extra)
	if err != nil {
		return nil, fmt.Errorf("could not get image bytes from camera: %w", err)
	}
	if len(resBytes) == 0 {
		return nil, errors.New("received empty bytes from camera")
	}
	img, err := rimage.DecodeImage(ctx, resBytes, utils.WithLazyMIMEType(resMetadata.MimeType))
	if err != nil {
		return nil, fmt.Errorf("could not decode into image.Image: %w", err)
	}
	return img, nil
}

// A PointCloudSource is a source that can generate pointclouds.
type PointCloudSource interface {
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
}

// A ImagesSource is a source that can return a list of images with timestamp.
type ImagesSource interface {
	Images(ctx context.Context) ([]NamedImage, resource.ResponseMetadata, error)
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

// Package camera defines an image capturing device.
// For more information, see the [camera component docs].
//
// [camera component docs]: https://docs.viam.com/components/camera/
package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/camera/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
)

// ErrMIMETypeBytesMismatch indicates that the NamedImage's mimeType does not match the image bytes header.
//
// For example, if the image bytes are JPEG, but the mimeType is PNG, this error will be returned.
// This likely means there is a bug in the code that created the GetImages response.
//
// However, there may still be valid, decodeable underlying JPEG image bytes.
// If you want to decode the image bytes as a JPEG regardless of the mismatch, you can recover from this error,
// call the .Bytes() method, then decode the image bytes as JPEG manually with image.Decode().
var ErrMIMETypeBytesMismatch = errors.New("mime_type does not match the image bytes")

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
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
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
	data       []byte
	img        image.Image
	SourceName string
	mimeType   string
}

// NamedImageFromBytes constructs a NamedImage from a byte slice, source name, and mime type.
func NamedImageFromBytes(data []byte, sourceName, mimeType string) (NamedImage, error) {
	if data == nil {
		return NamedImage{}, fmt.Errorf("must provide image bytes to construct a named image from bytes")
	}
	if mimeType == "" {
		return NamedImage{}, fmt.Errorf("must provide a mime type to construct a named image")
	}
	return NamedImage{data: data, SourceName: sourceName, mimeType: mimeType}, nil
}

// NamedImageFromImage constructs a NamedImage from an image.Image, source name, and mime type.
func NamedImageFromImage(img image.Image, sourceName, mimeType string) (NamedImage, error) {
	if img == nil {
		return NamedImage{}, fmt.Errorf("must provide image to construct a named image from image")
	}
	if mimeType == "" {
		return NamedImage{}, fmt.Errorf("must provide a mime type to construct a named image")
	}
	return NamedImage{img: img, SourceName: sourceName, mimeType: mimeType}, nil
}

// Image returns the image.Image of the NamedImage.
func (ni *NamedImage) Image(ctx context.Context) (image.Image, error) {
	if ni.img != nil {
		return ni.img, nil
	}
	if ni.data == nil {
		return nil, fmt.Errorf("no image or image bytes available")
	}

	reader := bytes.NewReader(ni.data)
	_, header, err := image.DecodeConfig(reader)
	if err != nil {
		return nil, fmt.Errorf("could not decode image config: %w", err)
	}

	if header != "" && !strings.Contains(ni.mimeType, header) {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrMIMETypeBytesMismatch, ni.mimeType, header)
	}

	img, err := rimage.DecodeImage(ctx, ni.data, ni.mimeType)
	if err != nil {
		return nil, fmt.Errorf("could not decode bytes into image.Image: %w", err)
	}
	ni.img = img
	return ni.img, nil
}

// Bytes returns the byte slice of the NamedImage.
func (ni *NamedImage) Bytes(ctx context.Context) ([]byte, error) {
	if ni.data != nil {
		return ni.data, nil
	}
	if ni.img == nil {
		return nil, fmt.Errorf("no image or image bytes available")
	}

	data, err := rimage.EncodeImage(ctx, ni.img, ni.mimeType)
	if err != nil {
		return nil, fmt.Errorf("could not encode image with encoding %s: %w", ni.mimeType, err)
	}
	ni.data = data
	return ni.data, nil
}

// MimeType returns the mime type of the NamedImage.
func (ni *NamedImage) MimeType() string {
	return ni.mimeType
}

// ImageMetadata contains useful information about returned image bytes such as its mimetype.
type ImageMetadata struct {
	MimeType string
}

// A Camera is a resource that can capture frames.
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
//	images, metadata, err := myCamera.Images(context.Background(), nil)
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
type Camera interface {
	resource.Resource
	resource.Shaped

	// Images is used for getting simultaneous images from different imagers,
	// along with associated metadata (just timestamp for now). It's not for getting a time series of images from the same imager.
	// The extra parameter can be used to pass additional options to the camera resource. The filterSourceNames parameter can be used to filter
	// only the images from the specified source names. When unspecified, all images are returned.
	Images(ctx context.Context, filterSourceNames []string, extra map[string]interface{}) ([]NamedImage, resource.ResponseMetadata, error)

	// NextPointCloud returns the next immediately available point cloud, not necessarily one
	// a part of a sequence. In the future, there could be streaming of point clouds.
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)

	// Properties returns properties that are intrinsic to the particular
	// implementation of a camera.
	Properties(ctx context.Context) (Properties, error)
}

// VideoSource is a camera that has `Stream` embedded to directly integrate with gostream.
// Note that generally, when writing camera components from scratch, embedding `Stream` is an anti-pattern.
type VideoSource interface {
	Camera
	Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error)
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
	Images(ctx context.Context, filterSourceNames []string, extra map[string]interface{}) ([]NamedImage, resource.ResponseMetadata, error)
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

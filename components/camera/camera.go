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
	"time"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/camera/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
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
	data        []byte
	img         image.Image
	SourceName  string
	mimeType    string
	Annotations data.Annotations
}

// NamedImageFromBytes constructs a NamedImage from a byte slice, source name, mime type, and annotations.
func NamedImageFromBytes(data []byte, sourceName, mimeType string, annotations data.Annotations,
) (NamedImage, error) {
	if data == nil {
		return NamedImage{}, fmt.Errorf("must provide image bytes to construct a named image from bytes")
	}
	if mimeType == "" {
		return NamedImage{}, fmt.Errorf("must provide a mime type to construct a named image")
	}
	return NamedImage{data: data, SourceName: sourceName, mimeType: mimeType, Annotations: annotations}, nil
}

// NamedImageFromImage constructs a NamedImage from an image.Image, source name, mime type, and annotations.
func NamedImageFromImage(img image.Image, sourceName, mimeType string, annotations data.Annotations,
) (NamedImage, error) {
	if img == nil {
		return NamedImage{}, fmt.Errorf("must provide image to construct a named image from image")
	}
	if mimeType == "" {
		mimeType = utils.MimeTypeJPEG
	}
	return NamedImage{img: img, SourceName: sourceName, mimeType: mimeType, Annotations: annotations}, nil
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

// ImageMetadata contains useful information about returned image bytes such as its mimetype
// and any annotations associated with the image.
type ImageMetadata struct {
	MimeType    string
	Annotations data.Annotations
}

// A Camera is a resource that can capture frames.
// For more information, see the [camera component docs].
//
// Image example:
//
//	myCamera, err := camera.FromProvider(machine, "my_camera")
//	imageBytes, mimeType, err := myCamera.Image(context.Background(), utils.MimeTypeJPEG, nil)
//
// Or try to directly decode as an image.Image:
//
//	myCamera, err := camera.FromProvider(machine, "my_camera")
//	img, err = camera.DecodeImageFromCamera(context.Background(), utils.MimeTypeJPEG, nil, myCamera)
//
// For more information, see the [Image method docs].
//
// Images example:
//
//	myCamera, err := camera.FromProvider(machine, "my_camera")
//
//	images, metadata, err := myCamera.Images(context.Background(), nil, nil)
//
// For more information, see the [Images method docs].
//
// NextPointCloud example:
//
//	myCamera, err := camera.FromProvider(machine, "my_camera")
//
//	// gets the next point cloud from a camera
//	pointCloud, err := myCamera.NextPointCloud(context.Background(), nil)
//
// For more information, see the [NextPointCloud method docs].
//
// Close example:
//
//	myCamera, err := camera.FromProvider(machine, "my_camera")
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

	// Image returns a byte slice representing an image that tries to adhere to the MIME type hint.
	// Image also may return metadata about the frame.
	Image(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, ImageMetadata, error)

	// Images is used for getting simultaneous images from different imagers,
	// along with associated metadata (just timestamp for now). It's not for getting a time series of images from the same imager.
	// The extra parameter can be used to pass additional options to the camera resource. The filterSourceNames parameter can be used to filter
	// only the images from the specified source names. When unspecified, all images are returned.
	Images(ctx context.Context, filterSourceNames []string, extra map[string]interface{}) ([]NamedImage, resource.ResponseMetadata, error)

	// NextPointCloud returns the next immediately available point cloud, not necessarily one
	// a part of a sequence. In the future, there could be streaming of point clouds.
	NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error)

	// Properties returns properties that are intrinsic to the particular
	// implementation of a camera.
	Properties(ctx context.Context) (Properties, error)
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

// GetImageFromGetImages will be deprecated after RSDK-11726.
// It is a utility function to quickly implement GetImage from an already-implemented GetImages method.
// It returns a byte slice and ImageMetadata, which is the same response signature as the Image method.
//
// If sourceName is nil, it returns the first image in the response slice.
// If sourceName is not nil, it returns the image with the matching source name.
// If no image is found with the matching source name, it returns an error.
//
// It uses the mimeType arg to specify how to encode the bytes returned from GetImages.
// The extra parameter is passed through to the underlying Images method.
func GetImageFromGetImages(
	ctx context.Context,
	sourceName *string,
	cam Camera,
	extra map[string]interface{},
	filterSourceNames []string,
) ([]byte, ImageMetadata, error) {
	sourceNames := []string{}
	if sourceName != nil {
		sourceNames = append(sourceNames, *sourceName)
	}
	namedImages, _, err := cam.Images(ctx, sourceNames, extra)
	if err != nil {
		return nil, ImageMetadata{}, fmt.Errorf("could not get images from camera: %w", err)
	}
	if len(namedImages) == 0 {
		return nil, ImageMetadata{}, errors.New("no images returned from camera")
	}

	var img image.Image
	var mimeType string
	var annotations data.Annotations
	if sourceName == nil {
		img, err = namedImages[0].Image(ctx)
		if err != nil {
			return nil, ImageMetadata{}, fmt.Errorf("could not get image from named image: %w", err)
		}
		mimeType = namedImages[0].MimeType()
		annotations = namedImages[0].Annotations
	} else {
		for _, i := range namedImages {
			if i.SourceName == *sourceName {
				img, err = i.Image(ctx)
				if err != nil {
					return nil, ImageMetadata{}, fmt.Errorf("could not get image from named image: %w", err)
				}
				mimeType = i.MimeType()
				annotations = i.Annotations
				break
			}
		}
		if img == nil {
			return nil, ImageMetadata{}, errors.New("no image found with source name: " + *sourceName)
		}
	}

	if img == nil {
		return nil, ImageMetadata{}, errors.New("image is nil")
	}

	imgBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, ImageMetadata{}, fmt.Errorf("could not encode image with encoding %s: %w", mimeType, err)
	}
	return imgBytes, ImageMetadata{MimeType: mimeType, Annotations: annotations}, nil
}

// GetImagesFromGetImage will be deprecated after RSDK-11726.
// It is a utility function to quickly implement GetImages from an already-implemented GetImage method.
// It takes a mimeType, extra parameters, and a camera as args, and returns a slice of NamedImage and ResponseMetadata,
// which is the same response signature as the Images method. We use the mimeType arg to specify
// how to decode the image bytes returned from GetImage. The extra parameter is passed through to the underlying GetImage method.
// Source name is empty string always.
// It returns a slice of NamedImage of length 1 and ResponseMetadata, with empty string as the source name.
func GetImagesFromGetImage(
	ctx context.Context,
	mimeType string,
	cam Camera,
	logger logging.Logger,
	extra map[string]interface{},
) ([]NamedImage, resource.ResponseMetadata, error) {
	resBytes, resMetadata, err := cam.Image(ctx, mimeType, extra)
	if err != nil {
		return nil, resource.ResponseMetadata{}, fmt.Errorf("could not get image bytes from camera: %w", err)
	}
	if len(resBytes) == 0 {
		return nil, resource.ResponseMetadata{}, errors.New("received empty bytes from camera")
	}

	resMimetype, _ := utils.CheckLazyMIMEType(resMetadata.MimeType)
	reqMimetype, _ := utils.CheckLazyMIMEType(mimeType)
	if resMimetype != reqMimetype {
		logger.Warnf("requested mime type %s, but received %s", mimeType, resMimetype)
	}

	namedImg, err := NamedImageFromBytes(resBytes, "", resMetadata.MimeType, resMetadata.Annotations)
	if err != nil {
		return nil, resource.ResponseMetadata{}, fmt.Errorf("could not create named image: %w", err)
	}

	return []NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
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
	NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error)
}

// A ImagesSource is a source that can return a list of images with timestamp.
type ImagesSource interface {
	Images(ctx context.Context, filterSourceNames []string, extra map[string]interface{}) ([]NamedImage, resource.ResponseMetadata, error)
}

// NewPropertiesError returns an error specific to a failure in Properties.
func NewPropertiesError(cameraIdentifier string) error {
	return errors.Errorf("failed to get properties from %s", cameraIdentifier)
}

// Deprecated: FromDependencies is a helper for getting the named camera from a collection of
// dependencies. Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromDependencies(deps resource.Dependencies, name string) (Camera, error) {
	return resource.FromDependencies[Camera](deps, Named(name))
}

// Deprecated: FromRobot is a helper for getting the named Camera from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Camera, error) {
	return robot.ResourceFromRobot[Camera](r, Named(name))
}

// FromProvider is a helper for getting the named Camera from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Camera, error) {
	return resource.FromProvider[Camera](provider, Named(name))
}

// NamesFromRobot is a helper for getting all camera names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

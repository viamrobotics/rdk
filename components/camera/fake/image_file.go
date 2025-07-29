package fake

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"time"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
)

var fileModel = resource.DefaultModelFamily.WithModel("image_file")

func init() {
	resource.RegisterComponent(camera.API, fileModel,
		resource.Registration[camera.Camera, *fileSourceConfig]{
			Constructor: func(ctx context.Context, _ resource.Dependencies,
				conf resource.Config, logger logging.Logger,
			) (camera.Camera, error) {
				newConf, err := resource.NativeConfig[*fileSourceConfig](conf)
				if err != nil {
					return nil, err
				}
				return newCamera(context.Background(), conf.ResourceName(), newConf, logger)
			},
		})
}

func newCamera(ctx context.Context, name resource.Name, newConf *fileSourceConfig, logger logging.Logger) (camera.Camera, error) {
	videoSrc := &fileSource{
		ColorFN:        newConf.Color,
		DepthFN:        newConf.Depth,
		PointCloudFN:   newConf.PointCloud,
		Intrinsics:     newConf.CameraParameters,
		PreloadedImage: newConf.PreloadedImage,
		logger:         logger,
	}

	imgType := camera.ColorStream
	if newConf.Color == "" && newConf.PreloadedImage == "" {
		imgType = camera.DepthStream
	}
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(newConf.CameraParameters, newConf.DistortionParameters)
	src, err := camera.NewVideoSourceFromReader(
		ctx,
		videoSrc,
		&cameraModel,
		imgType,
	)
	if err != nil {
		return nil, err
	}
	return camera.FromVideoSource(name, src), nil
}

// fileSource stores the paths to a color and depth image and a pointcloud.
type fileSource struct {
	ColorFN        string
	DepthFN        string
	PointCloudFN   string
	Intrinsics     *transform.PinholeCameraIntrinsics
	PreloadedImage string
	logger         logging.Logger
}

// fileSourceConfig is the attribute struct for fileSource.
type fileSourceConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Color                string                             `json:"color_image_file_path,omitempty"`
	Depth                string                             `json:"depth_image_file_path,omitempty"`
	PointCloud           string                             `json:"pointcloud_file_path,omitempty"`
	PreloadedImage       string                             `json:"preloaded_image,omitempty"` // can be "pizza", "dog", or "crowd"
}

// Validate ensures all parts of the config are valid.
func (c fileSourceConfig) Validate(path string) ([]string, []string, error) {
	if c.CameraParameters != nil {
		if c.CameraParameters.Width < 0 || c.CameraParameters.Height < 0 {
			return nil, nil, fmt.Errorf(
				"got illegal negative dimensions for width_px and height_px (%d, %d) fields set in intrinsic_parameters for image_file camera",
				c.CameraParameters.Height, c.CameraParameters.Width)
		}
	}

	if c.PreloadedImage != "" {
		switch c.PreloadedImage {
		case "pizza", "dog", "crowd":
			// valid options
		default:
			return nil, nil, fmt.Errorf("preloaded_image must be one of: pizza, dog, crowd. Got: %s", c.PreloadedImage)
		}
	}

	return []string{}, nil, nil
}

// Read returns just the RGB image if it is present, or the depth map if the RGB image is not present.
func (fs *fileSource) Read(ctx context.Context) (image.Image, func(), error) {
	if fs.ColorFN == "" && fs.DepthFN == "" && fs.PreloadedImage == "" {
		return nil, nil, errors.New("no image file to read, so not implemented")
	}
	if fs.ColorFN == "" && fs.PreloadedImage == "" { // only depth info
		img, err := rimage.NewDepthMapFromFile(context.Background(), fs.DepthFN)
		if err != nil {
			return nil, nil, err
		}
		return img, func() {}, err
	}

	var img image.Image
	var err error
	// Get image from preloaded image or file
	if fs.PreloadedImage != "" {
		img, err = getPreloadedImage(fs.PreloadedImage)
	} else {
		img, err = rimage.ReadImageFromFile(fs.ColorFN)
	}

	if err != nil {
		return nil, nil, err
	}

	// x264 only supports even resolutions. Not every call to this function will
	// be in the context of an x264 stream, but we crop every image to even
	// dimensions anyways.
	oddWidth := img.Bounds().Dx()%2 != 0
	oddHeight := img.Bounds().Dy()%2 != 0
	if oddWidth || oddHeight {
		rImg := rimage.ConvertImage(img)
		newWidth := rImg.Width()
		newHeight := rImg.Height()
		if oddWidth {
			newWidth--
		}
		if oddHeight {
			newHeight--
		}
		img = rImg.SubImage(image.Rect(0, 0, newWidth, newHeight))
	}
	return img, func() {}, err
}

// Images returns the saved color and depth image if they are present.
func (fs *fileSource) Images(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	if fs.ColorFN == "" && fs.DepthFN == "" && fs.PreloadedImage == "" {
		return nil, resource.ResponseMetadata{}, errors.New("no image files to read, so not implemented")
	}
	imgs := []camera.NamedImage{}

	if fs.PreloadedImage != "" {
		img, err := getPreloadedImage(fs.PreloadedImage)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		imgs = append(imgs, camera.NamedImage{img, "preloaded"})
	}

	if fs.ColorFN != "" {
		img, err := rimage.ReadImageFromFile(fs.ColorFN)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		imgs = append(imgs, camera.NamedImage{img, "color"})
	}

	if fs.DepthFN != "" {
		dm, err := rimage.NewDepthMapFromFile(context.Background(), fs.DepthFN)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		imgs = append(imgs, camera.NamedImage{dm, "depth"})
	}

	ts := time.Now()
	return imgs, resource.ResponseMetadata{CapturedAt: ts}, nil
}

// NextPointCloud returns the point cloud from projecting the rgb and depth image using the intrinsic parameters,
// or the pointcloud from file if set.
func (fs *fileSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if fs.PointCloudFN != "" {
		return pointcloud.NewFromFile(fs.PointCloudFN, "")
	}
	if fs.Intrinsics == nil {
		return nil, transform.NewNoIntrinsicsError("camera intrinsics not found in config")
	}
	if fs.ColorFN == "" { // only depth info
		img, err := rimage.NewDepthMapFromFile(context.Background(), fs.DepthFN)
		if err != nil {
			return nil, err
		}
		return depthadapter.ToPointCloud(img, fs.Intrinsics), nil
	}
	img, err := rimage.NewImageFromFile(fs.ColorFN)
	if err != nil {
		return nil, err
	}
	dm, err := rimage.NewDepthMapFromFile(context.Background(), fs.DepthFN)
	if err != nil {
		return nil, err
	}
	return fs.Intrinsics.RGBDToPointCloud(img, dm)
}

func (fs *fileSource) Close(ctx context.Context) error {
	return nil
}

// StaticSource is a fixed, stored image. Used primarily for testing.
type StaticSource struct {
	ColorImg image.Image
	DepthImg image.Image
	Proj     transform.Projector
}

// Read returns the stored image.
func (ss *StaticSource) Read(ctx context.Context) (image.Image, func(), error) {
	if ss.ColorImg != nil {
		return ss.ColorImg, func() {}, nil
	}
	return ss.DepthImg, func() {}, nil
}

// Images returns the saved color and depth image if they are present.
func (ss *StaticSource) Images(ctx context.Context) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	if ss.ColorImg == nil && ss.DepthImg == nil {
		return nil, resource.ResponseMetadata{}, errors.New("no image files stored, so not implemented")
	}
	imgs := []camera.NamedImage{}
	if ss.ColorImg != nil {
		imgs = append(imgs, camera.NamedImage{ss.ColorImg, "color"})
	}
	if ss.DepthImg != nil {
		imgs = append(imgs, camera.NamedImage{ss.DepthImg, "depth"})
	}
	ts := time.Now()
	return imgs, resource.ResponseMetadata{CapturedAt: ts}, nil
}

// NextPointCloud returns the point cloud from projecting the rgb and depth image using the intrinsic parameters.
func (ss *StaticSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	if ss.Proj == nil {
		return nil, transform.NewNoIntrinsicsError("camera intrinsics not found in config")
	}
	if ss.DepthImg == nil {
		return nil, errors.New("no depth info to project to pointcloud")
	}
	col := rimage.ConvertImage(ss.ColorImg)
	dm, err := rimage.ConvertImageToDepthMap(context.Background(), ss.DepthImg)
	if err != nil {
		return nil, err
	}
	return ss.Proj.RGBDToPointCloud(col, dm)
}

// Close does nothing.
func (ss *StaticSource) Close(ctx context.Context) error {
	return nil
}

// getPreloadedImage returns one of the preloaded images based on the name.
func getPreloadedImage(name string) (*rimage.Image, error) {
	var imageBase64 []byte
	switch name {
	case "pizza":
		imageBase64 = pizzaBase64
	case "dog":
		imageBase64 = dogBase64
	case "crowd":
		imageBase64 = crowdBase64
	default:
		return nil, fmt.Errorf("unknown preloaded image: %s", name)
	}

	d := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(imageBase64))
	img, err := jpeg.Decode(d)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	return rimage.ConvertImage(img), nil
}

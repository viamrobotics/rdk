package videosource

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(camera.Subtype, "file",
		registry.Component{Constructor: func(ctx context.Context, _ registry.Dependencies,
			config config.Component, logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*fileSourceAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			videoSrc := &fileSource{attrs.Color, attrs.Depth, attrs.CameraParameters}
			return camera.NewFromReader(
				ctx,
				videoSrc,
				&transform.PinholeCameraModel{attrs.CameraParameters, attrs.DistortionParameters},
				camera.StreamType(attrs.Stream),
			)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "file",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf fileSourceAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*fileSourceAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&fileSourceAttrs{})
}

// fileSource stores the paths to a color and depth image.
type fileSource struct {
	ColorFN    string
	DepthFN    string
	Intrinsics *transform.PinholeCameraIntrinsics
}

// fileSourceAttrs is the attribute struct for fileSource.
type fileSourceAttrs struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Stream               string                             `json:"stream"`
	Debug                bool                               `json:"debug,omitempty"`
	Color                string                             `json:"color_file_path"`
	Depth                string                             `json:"depth_file_path"`
}

// Read returns just the RGB image if it is present, or the depth map if the RGB image is not present.
func (fs *fileSource) Read(ctx context.Context) (image.Image, func(), error) {
	if fs.ColorFN == "" { // only depth info
		img, err := rimage.NewDepthMapFromFile(context.Background(), fs.DepthFN)
		return img, func() {}, err
	}
	img, err := rimage.NewImageFromFile(fs.ColorFN)
	return img, func() {}, err
}

// NextPointCloud returns the point cloud from projecting the rgb and depth image using the intrinsic parameters.
func (fs *fileSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
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

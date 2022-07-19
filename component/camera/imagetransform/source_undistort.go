package imagetransform

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"undistort",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			source, err := camera.FromDependencies(deps, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera for undistort (%s): %w", sourceName, err)
			}
			return newUndistortSource(ctx, source, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "undistort",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&camera.AttrConfig{})
}

// undistortSource will undistort the original image according to the Distortion parameters
// in AttrConfig.CameraParameters.
type undistortSource struct {
	original     gostream.ImageSource
	stream       camera.StreamType
	cameraParams *transform.PinholeCameraIntrinsics
}

func newUndistortSource(ctx context.Context, source camera.Camera, attrs *camera.AttrConfig) (camera.Camera, error) {
	proj := camera.GetProjector(ctx, attrs, source)
	intrinsics, ok := proj.(*transform.PinholeCameraIntrinsics)
	if !ok {
		return nil, transform.NewNoIntrinsicsError("")
	}
	imgSrc := &undistortSource{source, camera.StreamType(attrs.Stream), intrinsics}
	return camera.New(imgSrc, proj)
}

// Next undistorts the original image according to the camera parameters.
func (us *undistortSource) Next(ctx context.Context) (image.Image, func(), error) {
	orig, release, err := us.original.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer release()
	switch us.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		color := rimage.ConvertImage(orig)
		color, err = us.cameraParams.UndistortImage(color)
		if err != nil {
			return nil, nil, err
		}
		return color, func() {}, nil
	case camera.DepthStream:
		depth, err := rimage.ConvertImageToDepthMap(orig)
		if err != nil {
			return nil, nil, err
		}
		depth, err = us.cameraParams.UndistortDepthMap(depth)
		if err != nil {
			return nil, nil, err
		}
		return rimage.MakeImageWithDepth(rimage.ConvertImage(depth.ToGray16Picture()), depth, true), func() {}, nil
	case camera.BothStream:
		both := rimage.ConvertToImageWithDepth(orig)
		color, err := us.cameraParams.UndistortImage(both.Color)
		if err != nil {
			return nil, nil, err
		}
		depth, err := us.cameraParams.UndistortDepthMap(both.Depth)
		if err != nil {
			return nil, nil, err
		}
		return rimage.MakeImageWithDepth(color, depth, both.IsAligned()), func() {}, nil
	default:
		return nil, nil, errors.Errorf("do not know how to decode stream type %q", string(us.stream))
	}
}

// Close closes the imageSource.
func (us *undistortSource) Close(ctx context.Context) error {
	return utils.TryClose(ctx, us.original)
}

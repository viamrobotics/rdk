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
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

var modelUndistort = resource.NewDefaultModel("undistort")


func init() {
	registry.RegisterComponent(
		camera.Subtype,
		modelUndistort,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
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

	config.RegisterComponentAttributeMapConverter(camera.Subtype, modelUndistort,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&transformConfig{})
}

// undistortSource will undistort the original image according to the Distortion parameters
// in AttrConfig.CameraParameters.
type undistortSource struct {
	original     gostream.ImageSource
	stream       camera.StreamType
	cameraParams *transform.PinholeCameraIntrinsics
}

func newUndistortSource(ctx context.Context, source camera.Camera, attrs *transformConfig) (camera.Camera, error) {
	proj, _ := camera.GetProjector(ctx, nil, source)
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
	switch us.stream {
	case camera.ColorStream, camera.UnspecifiedStream:
		color := rimage.ConvertImage(orig)
		color, err = us.cameraParams.UndistortImage(color)
		if err != nil {
			return nil, nil, err
		}
		return color, release, nil
	case camera.DepthStream:
		depth, err := rimage.ConvertImageToDepthMap(orig)
		if err != nil {
			return nil, nil, err
		}
		depth, err = us.cameraParams.UndistortDepthMap(depth)
		if err != nil {
			return nil, nil, err
		}
		return depth, release, nil
	default:
		return nil, nil, errors.Errorf("do not know how to decode stream type %q", string(us.stream))
	}
}

// Close closes the imageSource.
func (us *undistortSource) Close(ctx context.Context) error {
	return utils.TryClose(ctx, us.original)
}

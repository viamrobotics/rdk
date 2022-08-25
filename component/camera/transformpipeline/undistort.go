package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

type undistortConfig struct {
	CameraParams *transform.PinholeCameraIntrinsics `json:"camera_parameters"`
}

// undistortSource will undistort the original image according to the Distortion parameters
// within the intrinsic parameters.
type undistortSource struct {
	original     gostream.ImageSource
	stream       camera.StreamType
	cameraParams *transform.PinholeCameraIntrinsics
}

func newUndistortTransform(
	source gostream.ImageSource, stream camera.StreamType, am config.AttributeMap,
) (gostream.ImageSource, error) {
	conf, err := config.TransformAttributeMapToStruct(&(undistortConfig{}), am)
	if err != nil {
		return nil, err
	}
	attrs, ok := conf.(*undistortConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attrs, conf)
	}
	if attrs.CameraParams == nil {
		return nil, errors.Wrapf(transform.ErrNoIntrinsics, "cannot create undistort transform")
	}
	return &undistortSource{source, stream, attrs.CameraParams}, nil
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

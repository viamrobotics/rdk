package transformpipeline

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	rdkutils "go.viam.com/rdk/utils"
)

type undistortConfig struct {
	CameraParams     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	DistortionParams *transform.BrownConrady            `json:"distortion_parameters"`
}

// undistortSource will undistort the original image according to the Distortion parameters
// within the intrinsic parameters.
type undistortSource struct {
	originalStream gostream.VideoStream
	stream         camera.StreamType
	cameraParams   *transform.PinholeCameraModel
}

func newUndistortTransform(
	ctx context.Context, source gostream.VideoSource, stream camera.StreamType, am config.AttributeMap,
) (gostream.VideoSource, error) {
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
	reader := &undistortSource{
		gostream.NewEmbeddedVideoStream(source),
		stream,
		&transform.PinholeCameraModel{attrs.CameraParams, attrs.DistortionParams},
	}
	return camera.NewFromReader(ctx, reader, nil, stream)
}

// Read undistorts the original image according to the camera parameters.
func (us *undistortSource) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::undistort::Read")
	defer span.End()
	orig, release, err := us.originalStream.Next(ctx)
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
		depth, err := rimage.ConvertImageToDepthMap(ctx, orig)
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

// Close closes the original stream.
func (us *undistortSource) Close(ctx context.Context) error {
	return us.originalStream.Close(ctx)
}

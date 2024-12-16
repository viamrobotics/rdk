// Package transformpipeline defines image sources that apply transforms on images, and can be composed into
// an image transformation pipeline. The image sources are not original generators of image, but require an image source
// from a real camera or video in order to function.
package transformpipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	camerautils "go.viam.com/rdk/robot/web/stream/camera"
	"go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("transform")

func init() {
	resource.RegisterComponent(
		camera.API,
		model,
		resource.Registration[camera.Camera, *transformConfig]{
			DeprecatedRobotConstructor: func(
				ctx context.Context,
				r any,
				conf resource.Config,
				logger logging.Logger,
			) (camera.Camera, error) {
				actualR, err := utils.AssertType[robot.Robot](r)
				if err != nil {
					return nil, err
				}
				newConf, err := resource.NativeConfig[*transformConfig](conf)
				if err != nil {
					return nil, err
				}
				sourceName := newConf.Source
				source, err := camera.FromRobot(actualR, sourceName)
				if err != nil {
					return nil, fmt.Errorf("no source camera for transform pipeline (%s): %w", sourceName, err)
				}
				streamCamera := streamCameraFromCamera(ctx, source)
				src, err := newTransformPipeline(ctx, streamCamera, conf.ResourceName().AsNamed(), newConf, actualR, logger)
				if err != nil {
					return nil, err
				}
				return src, nil
			},
		})
}

// transformConfig specifies a stream and list of transforms to apply on images/pointclouds coming from a source camera.
type transformConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Source               string                             `json:"source"`
	Pipeline             []Transformation                   `json:"pipeline"`
}

// Validate ensures all parts of the config are valid.
func (cfg *transformConfig) Validate(path string) ([]string, error) {
	var deps []string
	if len(cfg.Source) == 0 {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "source")
	}

	if cfg.CameraParameters != nil {
		if cfg.CameraParameters.Height < 0 || cfg.CameraParameters.Width < 0 {
			return nil, errors.Errorf(
				"got illegal negative dimensions for width_px and height_px (%d, %d) fields set in intrinsic_parameters for transform camera",
				cfg.CameraParameters.Width, cfg.CameraParameters.Height,
			)
		}
	}

	deps = append(deps, cfg.Source)
	return deps, nil
}

type streamCamera struct {
	camera.Camera
	vs gostream.VideoSource
}

func (sc *streamCamera) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	if sc.vs != nil {
		return sc.vs.Stream(ctx, errHandlers...)
	}
	return sc.Stream(ctx, errHandlers...)
}

// streamCameraFromCamera is a hack to allow us to use Stream to pipe frames through the pipeline
// and still implement a camera resource.
// We prefer this methodology over passing Image bytes because each transform desires a image.Image over
// a raw byte slice. To use Image would be to wastefully encode and decode the frame multiple times.
func streamCameraFromCamera(ctx context.Context, cam camera.Camera) camera.StreamCamera {
	if streamCam, ok := cam.(camera.StreamCamera); ok {
		return streamCam
	}
	return &streamCamera{
		Camera: cam,
		vs:     camerautils.VideoSourceFromCamera(ctx, cam),
	}
}

func newTransformPipeline(
	ctx context.Context,
	source camera.StreamCamera,
	named resource.Named,
	cfg *transformConfig,
	r robot.Robot,
	logger logging.Logger,
) (camera.StreamCamera, error) {
	if source == nil {
		return nil, errors.New("no source camera for transform pipeline")
	}
	if len(cfg.Pipeline) == 0 {
		return nil, errors.New("pipeline has no transforms in it")
	}
	// check if the source produces a depth image or color image
	img, err := camera.DecodeImageFromCamera(ctx, "", nil, source)

	var streamType camera.ImageType
	if err != nil {
		streamType = camera.UnspecifiedStream
	} else if _, ok := img.(*rimage.DepthMap); ok {
		streamType = camera.DepthStream
	} else if _, ok := img.(*image.Gray16); ok {
		streamType = camera.DepthStream
	} else {
		streamType = camera.ColorStream
	}
	// loop through the pipeline and create the image flow
	pipeline := make([]camera.StreamCamera, 0, len(cfg.Pipeline))
	lastSource := streamCameraFromCamera(ctx, source)
	for _, tr := range cfg.Pipeline {
		src, newStreamType, err := buildTransform(ctx, r, lastSource, streamType, tr, cfg.Source)
		if err != nil {
			return nil, err
		}
		streamSrc := streamCameraFromCamera(ctx, src)
		pipeline = append(pipeline, streamSrc)
		lastSource = streamSrc
		streamType = newStreamType
	}
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(cfg.CameraParameters, cfg.DistortionParameters)
	return camera.NewVideoSourceFromReader(
		ctx,
		transformPipeline{named, pipeline, lastSource, cfg.CameraParameters, logger},
		&cameraModel,
		streamType,
	)
}

type transformPipeline struct {
	resource.Named
	pipeline            []camera.StreamCamera
	src                 camera.Camera
	intrinsicParameters *transform.PinholeCameraIntrinsics
	logger              logging.Logger
}

func (tp transformPipeline) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::Read")
	defer span.End()
	img, err := camera.DecodeImageFromCamera(ctx, "", nil, tp.src)
	if err != nil {
		return nil, func() {}, err
	}
	return img, func() {}, nil
}

func (tp transformPipeline) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::NextPointCloud")
	defer span.End()
	if lastElem, ok := tp.pipeline[len(tp.pipeline)-1].(camera.PointCloudSource); ok {
		pc, err := lastElem.NextPointCloud(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "function NextPointCloud not defined for last videosource in transform pipeline")
		}
		return pc, nil
	}
	return nil, errors.New("function NextPointCloud not defined for last videosource in transform pipeline")
}

func (tp transformPipeline) Close(ctx context.Context) error {
	var errs error
	for _, src := range tp.pipeline {
		errs = multierr.Combine(errs, func() (err error) {
			defer func() {
				if panicErr := recover(); panicErr != nil {
					err = multierr.Combine(err, errors.Errorf("panic: %v", panicErr))
				}
			}()
			return src.Close(ctx)
		}())
	}
	return multierr.Combine(tp.src.Close(ctx), errs)
}

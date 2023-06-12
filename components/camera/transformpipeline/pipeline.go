// Package transformpipeline defines image sources that apply transforms on images, and can be composed into
// an image transformation pipeline. The image sources are not original generators of image, but require an image source
// from a real camera or video in order to function.
package transformpipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
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
				logger golog.Logger,
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
				src, err := newTransformPipeline(ctx, source, newConf, actualR)
				if err != nil {
					return nil, err
				}
				return camera.FromVideoSource(conf.ResourceName(), src), nil
			},
		})
}

// transformConfig specifies a stream and list of transforms to apply on images/pointclouds coming from a source camera.
type transformConfig struct {
	resource.TriviallyValidateConfig
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters,omitempty"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters,omitempty"`
	Debug                bool                               `json:"debug,omitempty"`
	Source               string                             `json:"source"`
	Pipeline             []Transformation                   `json:"pipeline"`
}

// Validate ensures all parts of the config are valid.
func (cfg *transformConfig) Validate(path string) ([]string, error) {
	var deps []string
	if len(cfg.Source) == 0 {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "source")
	}
	deps = append(deps, cfg.Source)
	return deps, nil
}

func newTransformPipeline(
	ctx context.Context,
	source gostream.VideoSource,
	cfg *transformConfig,
	r robot.Robot,
) (camera.VideoSource, error) {
	if source == nil {
		return nil, errors.New("no source camera for transform pipeline")
	}
	if len(cfg.Pipeline) == 0 {
		return nil, errors.New("pipeline has no transforms in it")
	}
	// check if the source produces a depth image or color image
	img, release, err := camera.ReadImage(ctx, source)
	if err != nil {
		return nil, err
	}
	var streamType camera.ImageType
	if _, ok := img.(*rimage.DepthMap); ok {
		streamType = camera.DepthStream
	} else if _, ok := img.(*image.Gray16); ok {
		streamType = camera.DepthStream
	} else {
		streamType = camera.ColorStream
	}
	release()
	// loop through the pipeline and create the image flow
	pipeline := make([]gostream.VideoSource, 0, len(cfg.Pipeline))
	lastSource := source
	for _, tr := range cfg.Pipeline {
		src, newStreamType, err := buildTransform(ctx, r, lastSource, streamType, tr)
		if err != nil {
			return nil, err
		}
		pipeline = append(pipeline, src)
		lastSource = src
		streamType = newStreamType
	}
	lastSourceStream := gostream.NewEmbeddedVideoStream(lastSource)
	cameraModel := camera.NewPinholeModelWithBrownConradyDistortion(cfg.CameraParameters, cfg.DistortionParameters)
	return camera.NewVideoSourceFromReader(
		ctx,
		transformPipeline{pipeline, lastSourceStream, cfg.CameraParameters},
		&cameraModel,
		streamType,
	)
}

type transformPipeline struct {
	pipeline            []gostream.VideoSource
	stream              gostream.VideoStream
	intrinsicParameters *transform.PinholeCameraIntrinsics
}

func (tp transformPipeline) Read(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "camera::transformpipeline::Read")
	defer span.End()
	return tp.stream.Next(ctx)
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
	return multierr.Combine(tp.stream.Close(ctx), errs)
}

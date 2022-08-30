// Package transformpipeline defines image sources that apply transforms on images, and can be composed into
// an image transformation pipeline. The image sources are not original generators of image, but require an image source
// from a real camera or video in order to function.
package transformpipeline

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"transform",
		registry.Component{RobotConstructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			source, err := camera.FromRobot(r, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera for transform pipeline (%s): %w", sourceName, err)
			}
			return newTransformPipeline(ctx, source, attrs, r)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "transform",
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf transformConfig
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*transformConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		},
		&transformConfig{})
}

// transformConfig specifies a stream and list of transforms to apply on images/pointclouds coming from a source camera.
type transformConfig struct {
	*camera.AttrConfig
	Source   string           `json:"source"`
	Pipeline []Transformation `json:"pipeline"`
}

func newTransformPipeline(
	ctx context.Context, source gostream.VideoSource, cfg *transformConfig, r robot.Robot,
) (camera.Camera, error) {
	if source == nil {
		return nil, errors.New("no source camera for transform pipeline")
	}
	if len(cfg.Pipeline) == 0 {
		return nil, errors.New("pipeline has no transforms in it")
	}
	// loop through the pipeline and create the image flow
	pipeline := make([]gostream.VideoSource, 0, len(cfg.Pipeline))
	lastSource := source
	for _, tr := range cfg.Pipeline {
		src, err := buildTransform(ctx, r, lastSource, cfg, tr)
		if err != nil {
			return nil, err
		}
		pipeline = append(pipeline, src)
		lastSource = src
	}
	lastSourceStream := gostream.NewEmbeddedVideoStream(lastSource)
	proj, _ := camera.GetProjector(ctx, cfg.AttrConfig, nil)
	return camera.NewFromReader(transformPipeline{pipeline, lastSourceStream}, proj)
}

type transformPipeline struct {
	pipeline []gostream.VideoSource
	stream   gostream.VideoStream
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

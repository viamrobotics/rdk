// Package transformpipeline defines cameras that apply transforms for images in an image transformation pipeline.
// They are not original generators of image, but require an image source in order to function.
// They are typically registered as cameras in the API.
package transformpipeline

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"transform",
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
				return nil, fmt.Errorf("no source camera for transform pipeline (%s): %w", sourceName, err)
			}
			return newTransformPipeline(ctx, source, attrs)
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

func newTransformPipeline(ctx context.Context, source camera.Camera, cfg *transformConfig) (camera.Camera, error) {
	if source == nil {
		return nil, errors.New("no source camera for transform pipeline")
	}
	stream := camera.StreamType(cfg.Stream)
	// loop through the pipeline and create the cameras
	outCam := source
	for _, tr := range cfg.Pipeline {
		cam, err := buildTransform(ctx, outCam, stream, tr)
		if err != nil {
			return nil, err
		}
		outCam = cam
	}
	proj, _ := camera.GetProjector(ctx, cfg.AttrConfig, outCam)
	return camera.New(outCam, proj)
}

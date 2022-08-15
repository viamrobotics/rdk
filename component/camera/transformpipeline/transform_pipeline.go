// Package transformpipeline defines cameras that apply transforms for images in an image transformation pipeline.
// They are not original generators of image, but require an image source in order to function.
// They are typically registered as cameras in the API.
package transformpipeline

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
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
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&transformConfig{})
}

// Transformation states the type of transformation and the attributes that are specific to the given type.
type Transformation struct {
	Type       string              `json:"type"`
	Attributes config.AttributeMap `json:"attributes"`
}

// transformConfig specifies a stream and list of transforms to apply on images/pointclouds coming from a source camera.
type transformConfig struct {
	Source   string           `json:"source"`
	Stream   string           `json:"stream"`
	Pipeline []Transformation `json:"pipeline"`
}

func newTransformPipeline(ctx context.Context, source camera.Camera, attrs *transformConfig) (camera.Camera, error) {
	return nil, nil
}

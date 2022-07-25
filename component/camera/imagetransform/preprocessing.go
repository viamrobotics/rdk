package imagetransform

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"preprocess_depth",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newPreprocessDepth(ctx, deps, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "preprocess_depth",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.AttrConfig{})
}

// preprocessDepthSource applies pre-processing functions to depth maps in order to smooth edges and fill holes.
type preprocessDepthSource struct {
	source gostream.ImageSource
}

// Next applies depth preprocessing to the next image.
func (os *preprocessDepthSource) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := os.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("no depth")
	}
	ii.Depth, err = rimage.PreprocessDepthMap(ii.Depth, ii.Color)
	if ii.Depth == nil {
		return nil, nil, err
	}
	return ii, func() {}, nil
}

func newPreprocessDepth(ctx context.Context, deps registry.Dependencies, attrs *camera.AttrConfig) (camera.Camera, error) {
	source, err := camera.FromDependencies(deps, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	imgSrc := &preprocessDepthSource{source}
	proj, _ := camera.GetProjector(ctx, attrs, source)
	return camera.New(imgSrc, proj)
}

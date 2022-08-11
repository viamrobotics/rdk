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
			attrs, ok := config.ConvertedAttributes.(*transformConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newPreprocessDepth(ctx, deps, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "preprocess_depth",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf transformConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &transformConfig{})
}

// preprocessDepthSource applies pre-processing functions to depth maps in order to smooth edges and fill holes.
type preprocessDepthSource struct {
	source gostream.ImageSource
}

// Next applies depth preprocessing to the next image.
func (os *preprocessDepthSource) Next(ctx context.Context) (image.Image, func(), error) {
	i, release, err := os.source.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	dm, err := rimage.ConvertImageToDepthMap(i)
	if err != nil {
		return nil, nil, errors.Wrap(err, "camera does not provide depth image")
	}
	dm, err = rimage.PreprocessDepthMap(dm, nil)
	if err != nil {
		return nil, nil, err
	}
	return dm, release, nil
}

func newPreprocessDepth(ctx context.Context, deps registry.Dependencies, attrs *transformConfig) (camera.Camera, error) {
	source, err := camera.FromDependencies(deps, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	imgSrc := &preprocessDepthSource{source}
	proj, _ := camera.GetProjector(ctx, nil, source)
	return camera.New(imgSrc, proj)
}

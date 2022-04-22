package imagesource

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
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"preprocess_depth",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newPreprocessDepth(r, attrs)
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
	ii, err = rimage.PreprocessDepthMap(ii)
	if ii.Depth == nil {
		return nil, nil, err
	}
	return ii, func() {}, nil
}

func newPreprocessDepth(r robot.Robot, attrs *camera.AttrConfig) (camera.MinimalCamera, error) {
	source, err := camera.FromRobot(r, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	imgSrc := &preprocessDepthSource{source}
	return camera.New(imgSrc, attrs, source)
}

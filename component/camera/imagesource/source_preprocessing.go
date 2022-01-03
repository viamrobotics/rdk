package imagesource

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
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
			return newPreprocessDepth(r, config)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "preprocess_depth",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf rimage.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &rimage.AttrConfig{})
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

func newPreprocessDepth(r robot.Robot, config config.Component) (camera.Camera, error) {
	source, ok := r.CameraByName(config.Attributes.String("source"))
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", config.Attributes.String("source"))
	}
	return &camera.ImageSource{ImageSource: &preprocessDepthSource{source}}, nil
}

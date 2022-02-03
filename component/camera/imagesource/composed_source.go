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
		"depth_to_pretty",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newDepthToPretty(r, config.ConvertedAttributes.(*rimage.AttrConfig))
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "depth_to_pretty",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf rimage.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &rimage.AttrConfig{})

	registry.RegisterComponent(
		camera.Subtype,
		"overlay",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newOverlay(r, config.ConvertedAttributes.(*rimage.AttrConfig))
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "overlay",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf rimage.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &rimage.AttrConfig{})
}

type overlaySource struct {
	source gostream.ImageSource
}

func (os *overlaySource) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := os.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("no depth")
	}
	return ii.Overlay(), func() {}, nil
}

func newOverlay(r robot.Robot, attrs *rimage.AttrConfig) (camera.Camera, error) {
	source, ok := camera.FromRobot(r, attrs.Source)
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", attrs.Source)
	}
	return &camera.ImageSource{ImageSource: &overlaySource{source}}, nil
}

type depthToPretty struct {
	source gostream.ImageSource
}

func (dtp *depthToPretty) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := dtp.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("no depth")
	}
	return ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), func() {}, nil
}

func newDepthToPretty(r robot.Robot, attrs *rimage.AttrConfig) (camera.Camera, error) {
	source, ok := camera.FromRobot(r, attrs.Source)
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", attrs.Source)
	}
	return &camera.ImageSource{ImageSource: &depthToPretty{source}}, nil
}

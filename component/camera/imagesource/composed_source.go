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
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, errors.Errorf("expected config.ConvertedAttributes to be *camera.AttrConfig but got %T", config.ConvertedAttributes)
			}
			return newDepthToPretty(r, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "depth_to_pretty",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.AttrConfig{})

	registry.RegisterComponent(
		camera.Subtype,
		"overlay",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, errors.Errorf("expected config.ConvertedAttributes to be *camera.AttrConfig but got %T", config.ConvertedAttributes)
			}
			return newOverlay(r, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "overlay",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.AttrConfig{})
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

func newOverlay(r robot.Robot, attrs *camera.AttrConfig) (camera.Camera, error) {
	source, ok := r.CameraByName(attrs.Source)
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", attrs.Source)
	}
	imgSrc := &overlaySource{source}
	return camera.New(imgSrc, attrs, source)
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

func newDepthToPretty(r robot.Robot, attrs *camera.AttrConfig) (camera.Camera, error) {
	source, ok := r.CameraByName(attrs.Source)
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", attrs.Source)
	}
	imgSrc := &depthToPretty{source}
	return camera.New(imgSrc, attrs, source)
}

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
		"depth_to_pretty",
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
			return newDepthToPretty(r, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "depth_to_pretty",
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
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newOverlay(r, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "overlay",
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

func newOverlay(r robot.Robot, attrs *camera.AttrConfig) (camera.MinimalCamera, error) {
	source, err := camera.FromRobot(r, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
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
	return rimage.MakeImageWithDepth(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), ii.Depth, true), func() {}, nil
}

func newDepthToPretty(r robot.Robot, attrs *camera.AttrConfig) (camera.MinimalCamera, error) {
	source, err := camera.FromRobot(r, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	imgSrc := &depthToPretty{source}
	return camera.New(imgSrc, attrs, source)
}

package imagesource

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"rotate",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			source, ok := r.CameraByName(sourceName)
			if !ok {
				return nil, errors.Errorf("cannot find source camera for rotate (%s)", sourceName)
			}
			imgSrc := &rotateImageDepthSource{source}
			return camera.New(imgSrc, attrs, source)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "rotate",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&camera.AttrConfig{})

	registry.RegisterComponent(
		camera.Subtype,
		"resize",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			source, ok := r.CameraByName(sourceName)
			if !ok {
				return nil, errors.Errorf("cannot find source camera for resize (%s)", sourceName)
			}

			width := attrs.Width
			if width == 0 {
				width = 800
			}
			height := attrs.Height
			if height == 0 {
				height = 640
			}

			imgSrc := gostream.ResizeImageSource{Src: source, Width: width, Height: height}
			return camera.New(imgSrc, attrs, nil) // camera parameters from source camera do not work for resized images
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "resize",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&camera.AttrConfig{})
}

// rotateImageDepthSource TODO.
type rotateImageDepthSource struct {
	Original gostream.ImageSource
}

// Next TODO.
func (rids *rotateImageDepthSource) Next(ctx context.Context) (image.Image, func(), error) {
	orig, release, err := rids.Original.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer release()

	iwd, ok := orig.(*rimage.ImageWithDepth)
	if !ok {
		return imaging.Rotate(orig, 180, color.Black), func() {}, nil
	}

	return iwd.Rotate(180), func() {}, nil
}

// Close TODO.
func (rids *rotateImageDepthSource) Close(ctx context.Context) error {
	return utils.TryClose(ctx, rids.Original)
}

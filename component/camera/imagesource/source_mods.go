package imagesource

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
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
		"rotate",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			sourceName := config.Attributes.String("source")
			source, ok := r.CameraByName(sourceName)
			if !ok {
				return nil, errors.Errorf("cannot find source camera for rotate (%s)", sourceName)
			}

			return &camera.ImageSource{ImageSource: &rotateImageDepthSource{source}}, nil
		}})

	registry.RegisterComponent(
		camera.Subtype,
		"resize",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			sourceName := config.Attributes.String("source")
			source, ok := r.CameraByName(sourceName)
			if !ok {
				return nil, errors.Errorf("cannot find source camera for resize (%s)", sourceName)
			}

			width := config.Attributes.Int("width", 800)
			height := config.Attributes.Int("height", 640)

			return &camera.ImageSource{
				ImageSource: gostream.ResizeImageSource{Src: source, Width: width, Height: height},
			}, nil
		}})

}

// rotateImageDepthSource TODO
type rotateImageDepthSource struct {
	Original gostream.ImageSource
}

// Next TODO
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

// Close TODO
func (rids *rotateImageDepthSource) Close() error {
	return rids.Original.Close()
}

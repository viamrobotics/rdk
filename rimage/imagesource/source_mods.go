package imagesource

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterCamera("rotate", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
		sourceName := config.Attributes.String("source")
		source := r.CameraByName(sourceName)
		if source == nil {
			return nil, errors.Errorf("cannot find source camera for rotate (%s)", sourceName)
		}

		return &RotateImageDepthSource{source}, nil
	})

	registry.RegisterCamera("resize", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
		sourceName := config.Attributes.String("source")
		source := r.CameraByName(sourceName)
		if source == nil {
			return nil, errors.Errorf("cannot find source camera for resize (%s)", sourceName)
		}

		width := config.Attributes.Int("width", 800)
		height := config.Attributes.Int("height", 640)

		return gostream.ResizeImageSource{source, width, height}, nil
	})

}

// RotateImageDepthSource TODO
type RotateImageDepthSource struct {
	Original gostream.ImageSource
}

// Next TODO
func (rids *RotateImageDepthSource) Next(ctx context.Context) (image.Image, func(), error) {
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
func (rids *RotateImageDepthSource) Close() error {
	return rids.Original.Close()
}

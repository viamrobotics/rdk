package imagesource

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"
)

func init() {
	api.RegisterCamera("rotate", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		sourceName := config.Attributes.String("source")
		source := r.CameraByName(sourceName)
		if source == nil {
			return nil, fmt.Errorf("cannot find source camera for rotate (%s)", sourceName)
		}

		return &RotateImageDepthSource{source}, nil
	})

	api.RegisterCamera("resize", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		sourceName := config.Attributes.String("source")
		source := r.CameraByName(sourceName)
		if source == nil {
			return nil, fmt.Errorf("cannot find source camera for resize (%s)", sourceName)
		}

		width := config.Attributes.Int("width", 800)
		height := config.Attributes.Int("height", 640)

		return gostream.ResizeImageSource{source, width, height}, nil
	})

}

type RotateImageDepthSource struct {
	Original gostream.ImageSource
}

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

func (rids *RotateImageDepthSource) Close() error {
	return rids.Original.Close()
}

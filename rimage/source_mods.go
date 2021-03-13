package rimage

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterCamera("rotate", func(r api.Robot, config api.Component) (gostream.ImageSource, error) {
		sourceName := config.Attributes.GetString("source")
		source := r.CameraByName(sourceName)
		if source == nil {
			return nil, fmt.Errorf("cannot find source camera for rotate (%s)", sourceName)
		}

		return &RotateImageDepthSource{source}, nil
	})

	api.RegisterCamera("resize", func(r api.Robot, config api.Component) (gostream.ImageSource, error) {
		sourceName := config.Attributes.GetString("source")
		source := r.CameraByName(sourceName)
		if source == nil {
			return nil, fmt.Errorf("cannot find source camera for resize (%s)", sourceName)
		}

		width := config.Attributes.GetInt("width", 800)
		height := config.Attributes.GetInt("height", 640)

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

	iwd, ok := orig.(*ImageWithDepth)
	if !ok {
		return imaging.Rotate(orig, 180, color.Black), func() {}, nil
	}

	return iwd.Rotate(180), func() {}, nil
}

func (rids *RotateImageDepthSource) Close() error {
	return rids.Original.Close()
}

package rimage

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/gostream"
)

type RotateImageDepthSource struct {
	Original gostream.ImageSource
}

func (rids *RotateImageDepthSource) Next(ctx context.Context) (image.Image, error) {
	orig, err := rids.Original.Next(ctx)
	if err != nil {
		return nil, err
	}

	iwd, ok := orig.(*ImageWithDepth)
	if !ok {
		return imaging.Rotate(orig, 180, color.Black), nil
	}

	return iwd.Rotate(180), nil
}

func (rids *RotateImageDepthSource) Close() error {
	return rids.Original.Close()
}

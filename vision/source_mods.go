package vision

import (
	"context"
	"image"
	"image/color"

	"github.com/disintegration/imaging"
	"github.com/edaniels/gostream"
	"gocv.io/x/gocv"
)

type RotateImageDepthSource struct {
	Original ImageDepthSource
}

func (rids *RotateImageDepthSource) Next(ctx context.Context) (image.Image, error) {
	rotateSrc := gostream.RotateImageSource{rids.Original}
	return rotateSrc.Next(ctx)
}

func (rids *RotateImageDepthSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	img, d, err := rids.Original.NextImageDepthPair(ctx)
	if err != nil {
		return nil, d, err
	}
	rotated := imaging.Rotate(img, 180, color.Black)

	if d != nil && d.HasData() {
		// TODO(erh): make this faster
		dm := d.ToMat()
		defer dm.Close()
		gocv.Rotate(dm, &dm, gocv.Rotate180Clockwise)
		d = NewDepthMapFromMat(dm)
	}

	return rotated, d, nil
}

func (rids *RotateImageDepthSource) Close() error {
	return rids.Original.Close()
}

package slam

import (
	"context"
	"image"
	"image/color"

	"github.com/fogleman/gg"
)

type AreaViewer struct {
	Area *SquareArea
}

func (av *AreaViewer) Next(ctx context.Context) (image.Image, error) {
	areaSize := av.Area.Dim()

	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?

	dc := gg.NewContext(areaSize, areaSize)

	av.Area.Mutate(func(area MutableArea) {
		offset := areaSize / 2
		area.Iterate(func(x, y, _ int) bool {
			dc.SetColor(color.RGBA{255, 0, 0, 255})
			dc.SetPixel(x+offset, y+offset)
			return true
		})
	})

	return dc.Image(), nil
}

func (av *AreaViewer) Close() error {
	return nil
}

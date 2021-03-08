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

func (av *AreaViewer) Next(ctx context.Context) (image.Image, func(), error) {
	areaSize := av.Area.Dim()

	dc := gg.NewContext(areaSize, areaSize)

	av.Area.Mutate(func(area MutableArea) {
		offset := areaSize / 2
		area.Iterate(func(x, y, _ int) bool {
			dc.SetColor(color.RGBA{255, 0, 0, 255})
			dc.SetPixel(x+offset, y+offset)
			return true
		})
	})

	return dc.Image(), func() {}, nil
}

func (av *AreaViewer) Close() error {
	return nil
}

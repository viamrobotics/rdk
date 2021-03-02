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
	areaX, areaY := av.Area.Dims()

	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?

	dc := gg.NewContext(areaX, areaY)

	av.Area.Mutate(func(area MutableArea) {
		area.Iterate(func(x, y, _ int) bool {
			dc.SetColor(color.RGBA{255, 0, 0, 255})
			dc.SetPixel(x, y)
			return true
		})
	})

	return dc.Image(), nil
}

func (av *AreaViewer) Close() error {
	return nil
}

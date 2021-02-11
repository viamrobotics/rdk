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
	areaSize, scaleDown := av.Area.Size()

	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?

	dc := gg.NewContext(areaSize*scaleDown, areaSize*scaleDown)

	av.Area.Mutate(func(area MutableArea) {
		area.DoNonZero(func(x, y int, _ float64) {
			dc.DrawPoint(float64(x), float64(y), 1)
			dc.SetColor(color.RGBA{255, 0, 0, 255})
			dc.Fill()
		})
	})

	return dc.Image(), nil
}

func (av *AreaViewer) Close() error {
	return nil
}

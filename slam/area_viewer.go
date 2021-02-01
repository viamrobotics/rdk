package slam

import (
	"context"
	"image"
	"image/color"

	"github.com/fogleman/gg"
)

type AreaViewer struct {
	Area      *SquareArea
	ViewScale int
}

func (av *AreaViewer) Next(ctx context.Context) (image.Image, error) {
	areaSize, areaSizeScale := av.Area.Size()
	areaSize *= areaSizeScale

	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?

	dc := gg.NewContext(areaSize/av.ViewScale, areaSize/av.ViewScale)

	av.Area.Mutate(func(area MutableArea) {
		area.DoNonZero(func(x, y int, _ float64) {
			dc.DrawPoint(float64(x/av.ViewScale), float64(y/av.ViewScale), 4)
			dc.SetColor(color.RGBA{255, 0, 0, 255})
			dc.Fill()
		})
	})

	return dc.Image(), nil
}

func (av *AreaViewer) Close() error {
	return nil
}

package slam

import (
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

type AreaViewer struct {
	Area      *SquareArea
	ViewScale int
}

func (av *AreaViewer) NextMat() (gocv.Mat, error) {
	areaSize, areaSizeScale := av.Area.Size()
	areaSize *= areaSizeScale

	// TODO(erd): any way to make this really fast? Allocate these in advance in
	// a goroutine? Pool?
	out := gocv.NewMatWithSize(areaSize/av.ViewScale, areaSize/av.ViewScale, gocv.MatTypeCV8UC3)

	av.Area.Mutate(func(area MutableArea) {
		area.DoNonZero(func(x, y int, _ float64) {
			p := image.Point{x / av.ViewScale, y / av.ViewScale}
			gocv.Circle(&out, p, 1, color.RGBA{R: 255}, 1)
		})
	})

	return out, nil
}

func (av *AreaViewer) Close() {
}

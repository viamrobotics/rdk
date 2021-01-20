package lidar

import (
	"image"
	"image/color"

	"github.com/echolabsinc/robotcore/vision"

	"gocv.io/x/gocv"
)

type MatSource struct {
	Device
}

func (ms MatSource) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	bounds, err := ms.Bounds()
	if err != nil {
		return gocv.Mat{}, vision.DepthMap{}, err
	}
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	measurements, err := ms.Scan()
	if err != nil {
		return gocv.Mat{}, vision.DepthMap{}, err
	}

	var drawLine bool
	// drawLine = true

	for _, next := range measurements {
		x, y := next.Coords()
		// m->cm
		scale := 100.0
		p := image.Point{centerX + int(x*scale), centerY + int(y*scale)} // scale to cm
		if drawLine {
			gocv.Line(&out, image.Point{centerX, centerY}, p, color.RGBA{R: 255}, 1)
		} else {
			gocv.Circle(&out, p, 4, color.RGBA{R: 255}, 1)
		}
	}

	return out, vision.DepthMap{}, nil
}

func (ms MatSource) Close() {}

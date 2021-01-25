package lidar

import (
	"image"
	"image/color"

	"github.com/echolabsinc/robotcore/vision"

	"gocv.io/x/gocv"
)

type MatSource struct {
	device    Device
	scaleDown int // scale down amount
}

const scaleDown = 100 // centimeters

func NewMatSource(device Device) *MatSource {
	return &MatSource{device: device, scaleDown: scaleDown}
}

func (ms *MatSource) NextColorDepthPair() (gocv.Mat, vision.DepthMap, error) {
	bounds, err := ms.device.Bounds()
	if err != nil {
		return gocv.Mat{}, vision.DepthMap{}, err
	}
	scaleDown := ms.scaleDown
	bounds.X *= scaleDown
	bounds.Y *= scaleDown
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	measurements, err := ms.device.Scan()
	if err != nil {
		return gocv.Mat{}, vision.DepthMap{}, err
	}

	var drawLine bool
	// drawLine = true

	for _, next := range measurements {
		x, y := next.Coords()
		p := image.Point{centerX + int(x*float64(scaleDown)), centerY + int(y*float64(scaleDown))}
		if drawLine {
			gocv.Line(&out, image.Point{centerX, centerY}, p, color.RGBA{R: 255}, 1)
		} else {
			gocv.Circle(&out, p, 4, color.RGBA{R: 255}, 1)
		}
	}

	return out, vision.DepthMap{}, nil
}

func (ms *MatSource) Close() {}

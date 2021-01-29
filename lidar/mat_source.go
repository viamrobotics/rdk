package lidar

import (
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

// MatSource generates images from the current scan of a lidar device
type MatSource struct {
	device    Device
	scaleDown int // scale down amount
}

const scaleDown = 100 // centimeters

func NewMatSource(device Device) *MatSource {
	return &MatSource{device: device, scaleDown: scaleDown}
}

func (ms *MatSource) NextMat() (gocv.Mat, error) {
	bounds, err := ms.device.Bounds()
	if err != nil {
		return gocv.Mat{}, err
	}
	scaleDown := ms.scaleDown
	bounds.X *= scaleDown
	bounds.Y *= scaleDown
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	measurements, err := ms.device.Scan()
	if err != nil {
		return gocv.Mat{}, err
	}

	for _, next := range measurements {
		x, y := next.Coords()
		p := image.Point{centerX + int(x*float64(scaleDown)), centerY + int(y*float64(scaleDown))}
		gocv.Circle(&out, p, 4, color.RGBA{R: 255}, 1)
	}

	return out, nil
}

func (ms *MatSource) Close() {}

package lidar

import (
	"context"
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

// ImageSource generates images from the current scan of a lidar device
type ImageSource struct {
	device    Device
	scaleDown int // scale down amount
}

const scaleDown = 100 // centimeters

func NewImageSource(device Device) *ImageSource {
	return &ImageSource{device: device, scaleDown: scaleDown}
}

func (is *ImageSource) Next(ctx context.Context) (image.Image, error) {
	bounds, err := is.device.Bounds()
	if err != nil {
		return nil, err
	}
	scaleDown := is.scaleDown
	bounds.X *= scaleDown
	bounds.Y *= scaleDown
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	measurements, err := is.device.Scan()
	if err != nil {
		return nil, err
	}

	for _, next := range measurements {
		x, y := next.Coords()
		p := image.Point{centerX + int(x*float64(scaleDown)), centerY + int(y*float64(scaleDown))}
		gocv.Circle(&out, p, 4, color.RGBA{R: 255}, 1)
	}

	return out.ToImage()
}

func (is *ImageSource) Close() error {
	return nil
}

package lidar

import (
	"context"
	"image"
	"image/color"

	"github.com/fogleman/gg"
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

	dc := gg.NewContext(bounds.X, bounds.Y)

	measurements, err := is.device.Scan()
	if err != nil {
		return nil, err
	}

	for _, next := range measurements {
		x, y := next.Coords()
		x = float64(centerX) + (x * float64(scaleDown))
		y = float64(centerY) + (y * float64(scaleDown))
		dc.DrawPoint(x, y, 4)
		dc.SetColor(color.RGBA{255, 0, 0, 255})
		dc.Fill()
	}

	return dc.Image(), nil
}

func (is *ImageSource) Close() error {
	return nil
}

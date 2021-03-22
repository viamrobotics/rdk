package lidar

import (
	"context"
	"image"

	"github.com/fogleman/gg"
	"go.viam.com/robotcore/rimage"
)

// ImageSource generates images from the current scan of a lidar device
type ImageSource struct {
	device        Device
	unitsPerMeter int
	noFilter      bool
}

const unitsPerMeter = 100 // centimeters

func NewImageSource(device Device) *ImageSource {
	return &ImageSource{device: device, unitsPerMeter: unitsPerMeter}
}

var red = rimage.Red

func (is *ImageSource) Next(ctx context.Context) (image.Image, func(), error) {
	bounds, err := is.device.Bounds(ctx)
	if err != nil {
		return nil, nil, err
	}
	unitsPerMeter := is.unitsPerMeter
	bounds.X *= unitsPerMeter
	bounds.Y *= unitsPerMeter
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	dc := gg.NewContext(bounds.X, bounds.Y)

	measurements, err := is.device.Scan(ctx, ScanOptions{NoFilter: is.noFilter})
	if err != nil {
		return nil, nil, err
	}

	for _, next := range measurements {
		x, y := next.Coords()
		x = float64(centerX) + (x * float64(unitsPerMeter))
		y = float64(centerY) + (y * float64(unitsPerMeter))
		dc.SetColor(red)
		dc.SetPixel(int(x), int(y))
	}

	return dc.Image(), func() {}, nil
}

func (is *ImageSource) Close() error {
	return nil
}

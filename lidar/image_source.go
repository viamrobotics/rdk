package lidar

import (
	"context"
	"image"
	"math"

	"github.com/fogleman/gg"
	"go.viam.com/robotcore/rimage"
)

// ImageSource generates images from the current scan of a lidar device
type ImageSource struct {
	Size          image.Point
	device        Device
	unitsPerMeter int
	noFilter      bool
}

const unitsPerMeter = 100 // centimeters

func NewImageSource(device Device) *ImageSource {
	return &ImageSource{Size: image.Point{800, 800}, device: device, unitsPerMeter: unitsPerMeter}
}

var red = rimage.Red

func (is *ImageSource) Next(ctx context.Context) (image.Image, func(), error) {
	measurements, err := is.device.Scan(ctx, ScanOptions{NoFilter: is.noFilter})
	if err != nil {
		return nil, nil, err
	}

	return MeasurementsToImage(measurements, is.Size), func() {}, nil
}

func (is *ImageSource) Close() error {
	return nil
}

func MeasurementsToImage(measurements Measurements, size image.Point) image.Image {

	if size.X != size.Y {
		panic("size has to be square")
	}
	maxDistance := .001

	for _, next := range measurements {
		if next.Distance() > maxDistance {
			maxDistance = next.Distance()
		}
	}

	// if maxDistance is 10 meters, and size is 100, 100
	// distance from center to edge is 50 pixels, which means we have
	// 5 pixels per meter
	pixelsPerMeter := float64(size.X/2) / maxDistance

	// round up to the next power of 2 to make it less jumpy
	pixelsPerMeter = math.Pow(2, math.Floor(math.Log2(pixelsPerMeter)))

	centerX := float64(size.X) / 2.0
	centerY := float64(size.Y) / 2.0

	dc := gg.NewContext(size.X, size.Y)

	for _, next := range measurements {
		x, y := next.Coords()
		x = centerX + (x * pixelsPerMeter)
		y = centerY + (y * -1 * pixelsPerMeter)
		dc.SetColor(red)
		dc.SetPixel(int(x), int(y))
	}

	return dc.Image()
}

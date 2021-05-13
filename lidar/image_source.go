package lidar

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/fogleman/gg"

	"go.viam.com/robotcore/rimage"
)

// An ImageSource generates images from the most recent scan of a lidar.
type ImageSource struct {
	size          image.Point
	device        Lidar
	unitsPerMeter int
	noFilter      bool
}

// unitsPerMeter helps size the resulting images.
const unitsPerMeter = 100 // centimeters

// NewImageSource returns a new image source that will produce images from the given device
// bounded to the given size.
func NewImageSource(outputSize image.Point, device Lidar) *ImageSource {
	return &ImageSource{size: outputSize, device: device, unitsPerMeter: unitsPerMeter}
}

// Next fetches the latest scan from the device and turns the measurements into
// an a properly sized image.
func (is *ImageSource) Next(ctx context.Context) (image.Image, func(), error) {
	measurements, err := is.device.Scan(ctx, ScanOptions{NoFilter: is.noFilter})
	if err != nil {
		return nil, nil, err
	}

	img, err := measurementsToImage(measurements, is.size)
	return img, func() {}, err
}

// Close does nothings since someone else is responsible for closing the underlying
// device.
func (is *ImageSource) Close() error {
	return nil
}

// measurementsToImage converts lidar measurements into an image bounded by the given size.
func measurementsToImage(measurements Measurements, size image.Point) (image.Image, error) {
	if size.X != size.Y {
		return nil, fmt.Errorf("size has to be square, not %v", size)
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
		dc.SetColor(rimage.Red)
		dc.SetPixel(int(x), int(y))
	}

	return dc.Image(), nil
}

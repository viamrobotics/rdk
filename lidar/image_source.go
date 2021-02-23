package lidar

import (
	"context"
	"image"
	"image/color"

	"github.com/viamrobotics/robotcore/utils"

	"github.com/fogleman/gg"
	"gonum.org/v1/gonum/mat"
)

// ImageSource generates images from the current scan of a lidar device
type ImageSource struct {
	device    Device
	scaleDown int // scale down amount
	noFilter  bool
}

const scaleDown = 100 // centimeters

func NewImageSource(device Device) *ImageSource {
	return &ImageSource{device: device, scaleDown: scaleDown}
}

func NewImageSourceNoFiltering(device Device) *ImageSource {
	return &ImageSource{device: device, scaleDown: scaleDown, noFilter: true}
}

var first *mat.Dense

func (is *ImageSource) Next(ctx context.Context) (image.Image, error) {
	bounds, err := is.device.Bounds(ctx)
	if err != nil {
		return nil, err
	}
	scaleDown := is.scaleDown
	bounds.X *= scaleDown
	bounds.Y *= scaleDown
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	dc := gg.NewContext(bounds.X, bounds.Y)

	measurements, err := is.device.Scan(ctx, ScanOptions{NoFilter: is.noFilter})
	if err != nil {
		return nil, err
	}

	if first == nil {
		measureMat := mat.NewDense(3, len(measurements), nil)
		for i, next := range measurements {
			x, y := next.Coords()
			measureMat.Set(0, i, x)
			measureMat.Set(1, i, y)
			measureMat.Set(2, i, 1)
		}
		first = measureMat
	}

	for _, next := range measurements {
		x, y := next.Coords()
		x = float64(centerX) + (x * float64(scaleDown))
		y = float64(centerY) + (y * float64(scaleDown))
		dc.SetColor(color.RGBA{255, 0, 0, 255})
		dc.SetPixel(int(x), int(y))
	}

	rotMat := (*mat.Dense)((*utils.Vec2Matrix)(first).RotateMatrixAbout(float64(0), float64(0), 90))
	_, numCols := rotMat.Dims()
	for i := 0; i < numCols; i++ {
		x := rotMat.At(0, i)
		y := rotMat.At(1, i)
		x = float64(centerX) + (x * float64(scaleDown))
		y = float64(centerY) + (y * float64(scaleDown))
		dc.SetColor(color.RGBA{0, 255, 0, 255})
		dc.SetPixel(int(x), int(y))
	}

	return dc.Image(), nil
}

func (is *ImageSource) Close() error {
	return nil
}

package slam

import (
	"context"
	"image"
	"image/color"
	"math"

	"github.com/fogleman/gg"
)

// AreaViewer produces images of an underlying square area.
type AreaViewer struct {
	Area *SquareArea
}

// Next returns the area as an image with based on the units.
func (av *AreaViewer) Next(ctx context.Context) (image.Image, func(), error) {
	return AreaToImage(av.Area), func() {}, nil
}

// Close does nothing.
func (av *AreaViewer) Close() error {
	return nil
}

// AreaToImage converts the area into an image with based on the units.
func AreaToImage(a *SquareArea) image.Image {
	areaSize := int(math.Round(a.Dim()))

	dc := gg.NewContext(areaSize, areaSize)

	a.Mutate(func(area MutableArea) {
		offset := areaSize / 2
		area.Iterate(func(x, y float64, _ int) bool {
			dc.SetColor(color.RGBA{255, 0, 0, 255})
			xi, yi := int(math.Round(x)), int(math.Round(y))
			dc.SetPixel(xi+offset, yi+offset)
			return true
		})
	})

	return dc.Image()
}

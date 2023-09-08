//go:build cgo
// Package rimage defines fundamental image and color processing primitives.
//
// The golang standard library, while useful, is not very productive when it
// comes to handling/comparing colors of different spaces, transforming images,
// and drawing images. This package aims to rectify these issues with a few
// unified interfaces/structures.
package rimage

import (
	"image"
	"image/color"
	"math"

	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/utils"
)

// Image is like image.Image but it uses our Color type with a few more
// helper methods on it.
type Image struct {
	data          []Color
	width, height int
}

// NewImage returns a blank new image of the given dimensions.
func NewImage(width, height int) *Image {
	return &Image{make([]Color, width*height), width, height}
}

// NewImageFromBounds returns blank new of dimensions defined by
// the given rectangle.
func NewImageFromBounds(bounds image.Rectangle) *Image {
	return NewImage(bounds.Max.X, bounds.Max.Y)
}

// ColorModel returns our Color types color model.
func (i *Image) ColorModel() color.Model {
	return &TheColorModel{}
}

// Clone makes a copy of the image.
func (i *Image) Clone() *Image {
	ii := NewImage(i.Width(), i.Height())
	copy(ii.data, i.data)
	return ii
}

// In returns whether or not a point is within bounds of this image.
func (i *Image) In(x, y int) bool {
	return x >= 0 && y >= 0 && x < i.width && y < i.height
}

func (i *Image) k(p image.Point) int {
	return i.kxy(p.X, p.Y)
}

func (i *Image) kxy(x, y int) int {
	return (y * i.width) + x
}

// Bounds returns the outer bounds of this image. Otherwise
// known as its dimensions.
func (i *Image) Bounds() image.Rectangle {
	return image.Rect(0, 0, i.width, i.height)
}

// Width returns the horizontal width of this image.
func (i *Image) Width() int {
	return i.width
}

// Height returns the vertical height of this image.
func (i *Image) Height() int {
	return i.height
}

// At returns the color at the given point; black if not set.
func (i *Image) At(x, y int) color.Color {
	return i.data[i.kxy(x, y)]
}

// Get returns the color at the given point; black if not set.
func (i *Image) Get(p image.Point) Color {
	return i.data[i.k(p)]
}

// GetXY returns the color at the given point; black if not set.
func (i *Image) GetXY(x, y int) Color {
	return i.data[i.kxy(x, y)]
}

// SetXY sets the color at the given point.
func (i *Image) SetXY(x, y int, c Color) {
	i.data[i.kxy(x, y)] = c
}

// Set sets the color at the given point.
func (i *Image) Set(p image.Point, c Color) {
	i.data[i.k(p)] = c
}

// WriteTo writes the image to the given file encoded based on the file
// extension.
func (i *Image) WriteTo(fn string) error {
	return WriteImageToFile(fn, i)
}

// Circle inscribes a circle centered at the given point.
func (i *Image) Circle(center image.Point, radius int, c Color) {
	err := utils.Walk(center.X, center.Y, radius, func(x, y int) error {
		if !i.In(x, y) {
			return nil
		}

		p := image.Point{x, y}
		if PointDistance(center, p) > float64(radius) {
			return nil
		}

		i.Set(p, c)
		return nil
	})
	if err != nil {
		panic(err) // impossible
	}
}

// SubImage returns a subset of the image defined by the given rectangle.
func (i *Image) SubImage(r image.Rectangle) *Image {
	if r.Empty() {
		return &Image{}
	}
	xmin, xmax := utils.MinInt(i.width, r.Min.X), utils.MinInt(i.width, r.Max.X)
	ymin, ymax := utils.MinInt(i.height, r.Min.Y), utils.MinInt(i.height, r.Max.Y)
	if xmin == xmax || ymin == ymax { // return empty Image
		return &Image{data: []Color{}, width: utils.MaxInt(0, xmax-xmin), height: utils.MaxInt(0, ymax-ymin)}
	}
	width := xmax - xmin
	height := ymax - ymin
	newData := make([]Color, 0, width*height)
	for y := ymin; y < ymax; y++ {
		begin, end := (y*i.width)+xmin, (y*i.width)+xmax
		newData = append(newData, i.data[begin:end]...)
	}
	return &Image{data: newData, width: width, height: height}
}

// ConvertColorImageToLuminanceFloat convert an Image to a gray level image as a float dense matrix.
func ConvertColorImageToLuminanceFloat(img *Image) *mat.Dense {
	out := mat.NewDense(img.height, img.width, nil)
	utils.ParallelForEachPixel(image.Point{img.width, img.height}, func(x, y int) {
		c := img.GetXY(x, y)
		l := math.Floor(Luminance(c))
		out.Set(y, x, l)
	})
	return out
}

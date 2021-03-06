package rimage

import (
	"image"
	"image/color"

	"go.viam.com/robotcore/utils"
)

type Image struct {
	data          []Color
	width, height int
}

func (i *Image) ColorModel() color.Model {
	return &TheColorModel{}
}

func (i *Image) In(x, y int) bool {
	return x >= 0 && y >= 0 && x < i.width && y < i.height
}

func (i *Image) k(p image.Point) int {
	return i.kxy(p.X, p.Y)
}

func (i *Image) kxy(x, y int) int {
	return (y * i.width) + x
}

func (i *Image) Bounds() image.Rectangle {
	return image.Rect(0, 0, i.width, i.height)
}

func (i *Image) Width() int {
	return i.width
}

func (i *Image) Height() int {
	return i.height
}

func (i *Image) At(x, y int) color.Color {
	return i.data[i.kxy(x, y)]
}

func (i *Image) Get(p image.Point) Color {
	return i.data[i.k(p)]
}

func (i *Image) GetXY(x, y int) Color {
	return i.data[i.kxy(x, y)]
}

func (i *Image) SetXY(x, y int, c Color) {
	i.data[i.kxy(x, y)] = c
}

func (i *Image) Set(p image.Point, c Color) {
	i.data[i.k(p)] = c
}

func (i *Image) WriteTo(fn string) error {
	return WriteImageToFile(fn, i)
}

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

func NewImage(width, height int) *Image {
	return &Image{make([]Color, width*height), width, height}
}

func NewImageFromBounds(bounds image.Rectangle) *Image {
	return NewImage(bounds.Max.X, bounds.Max.Y)
}

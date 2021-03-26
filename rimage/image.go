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

func (i *Image) SubImage(r image.Rectangle) Image {
	xmin, xmax := utils.MinInt(i.width, r.Min.X), utils.MinInt(i.width, r.Max.X)
	ymin, ymax := utils.MinInt(i.height, r.Min.Y), utils.MinInt(i.height, r.Max.Y)
	if xmin == xmax || ymin == ymax { // return empty Image
		return Image{data: []Color{}, width: utils.MaxInt(0, xmax-xmin), height: utils.MaxInt(0, ymax-ymin)}
	}
	width := xmax - xmin
	height := ymax - ymin
	newData := make([]Color, 0, width*height)
	for y := ymin; y < ymax; y++ {
		begin, end := (y*i.width)+xmin, (y*i.width)+xmax
		newData = append(newData, i.data[begin:end]...)
	}
	return Image{data: newData, width: width, height: height}
}

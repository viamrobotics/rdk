package rimage

import (
	"image"
	"image/color"
	"sync"

	"go.viam.com/robotcore/utils"
)

type Image struct {
	immutable     image.Image
	data          []Color
	width, height int
	mu            sync.Mutex
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
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.immutable != nil {
		return i.immutable.At(x, y)
	}
	return i.data[i.kxy(x, y)]
}

func (i *Image) Get(p image.Point) Color {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.immutable != nil {
		return NewColorFromColor(i.immutable.At(p.X, p.Y))
	}
	return i.data[i.k(p)]
}

func (i *Image) GetXY(x, y int) Color {
	return i.Get(image.Point{x, y})
}

func (i *Image) setXY(x, y int, c Color) {
	i.data[i.kxy(x, y)] = c
}

func (i *Image) SetXY(x, y int, c Color) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.makeMutable()
	i.setXY(x, y, c)
}

func (i *Image) Set(p image.Point, c Color) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.makeMutable()
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

func (i *Image) makeMutable() {
	if i.immutable == nil {
		return
	}
	i.data = make([]Color, i.width*i.height)
	for x := 0; x < i.width; x++ {
		for y := 0; y < i.height; y++ {
			i.setXY(x, y, NewColorFromColor(i.immutable.At(x, y)))
		}
	}
	i.immutable = nil
}

func NewImage(width, height int) *Image {
	return &Image{
		data:   make([]Color, width*height),
		width:  width,
		height: height,
	}
}

func NewImageFromBounds(bounds image.Rectangle) *Image {
	return NewImage(bounds.Max.X, bounds.Max.Y)
}

func NewImageFromStdImage(img image.Image) *Image {
	bounds := img.Bounds()
	return &Image{
		immutable: img,
		width:     bounds.Max.X,
		height:    bounds.Max.Y,
	}
}

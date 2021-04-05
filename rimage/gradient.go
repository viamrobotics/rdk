package rimage

import (
	"fmt"
	"image"
	"math"

	"gonum.org/v1/gonum/mat"
)

type Gradient struct {
	magnitude float64
	direction float64
}

type GradientField struct {
	width  int
	height int

	data []Gradient
}

func (g Gradient) Magnitude() float64 {
	return g.magnitude
}

func (g Gradient) Direction() float64 {
	return g.direction
}

func (gf *GradientField) kxy(x, y int) int {
	return (y * gf.width) + x
}

func (gf *GradientField) Width() int {
	return gf.width
}

func (gf *GradientField) Height() int {
	return gf.height
}

func (gf *GradientField) Get(p image.Point) Gradient {
	return gf.data[gf.kxy(p.X, p.Y)]
}

func (gf *GradientField) GetGradient(x, y int) Gradient {
	return gf.data[gf.kxy(x, y)]
}

func (gf *GradientField) Set(x, y int, val Gradient) {
	gf.data[gf.kxy(x, y)] = val
}

func MakeEmptyGradientField(width, height int) GradientField {
	gf := GradientField{
		width:  width,
		height: height,
		data:   make([]Gradient, width*height),
	}

	return gf
}

func MakeGradientFieldFromMat(magnitude, direction *mat.Dense) (*GradientField, error) {
	magH, magW := magnitude.Dims()
	dirH, dirW := direction.Dims()
	if magW != dirW && magH != dirH {
		return nil, fmt.Errorf("cannot make GradientField from two matrices of different sizes (%v,%v), (%v,%v).", magW, magH, dirW, dirH)
	}
	gf := MakeEmptyGradientField(dirW, dirH)
	for x := 0; x < dirW; x++ {
		for y := 0; y < dirH; y++ {
			gf.Set(x, y, Gradient{magnitude.At(y, x), direction.At(y, x)}) // in mat.Dense, indexing is (row, column)
		}
	}

	return &gf, nil
}

func (gf *GradientField) ToPrettyPicture() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, gf.Width(), gf.Height()))
	for x := 0; x < gf.Width(); x++ {
		for y := 0; y < gf.Height(); y++ {
			p := image.Point{x, y}
			g := gf.Get(p)
			if g.Magnitude() == 0 {
				continue
			}
			angle := g.Direction() * (180. / math.Pi)
			img.Set(x, y, NewColorFromHSV(angle, 1.0, 1.0))
		}
	}
	return img
}

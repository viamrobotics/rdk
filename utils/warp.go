package utils

import (
	"fmt"
	"image"
	"math"

	"gonum.org/v1/gonum/mat"

	"github.com/lucasb-eyer/go-colorful"

	"gocv.io/x/gocv"
)

type TransformationMatrix mat.Matrix

type WarpConnector interface {
	Get(x, y int, buf []float64)
	Set(x, y int, data []float64)
	OutputDims() (int, int)
	NumFields() int // how many float64 are in the buffers above
}

// -----

type WarpMatrixConnector struct {
	Input  mat.Matrix
	Output *mat.Dense
}

func (c *WarpMatrixConnector) Get(x, y int, buf []float64) {
	buf[0] = c.Input.At(x, y)
}

func (c *WarpMatrixConnector) Set(x, y int, data []float64) {
	c.Output.Set(x, y, data[0])
}

func (c *WarpMatrixConnector) OutputDims() (int, int) {
	return c.Output.Dims()
}

func (c *WarpMatrixConnector) NumFields() int {
	return 1
}

// -----

type WarpImageConnector struct {
	Input  image.Image
	Output *image.RGBA
}

func (c *WarpImageConnector) Get(x, y int, buf []float64) {
	temp, b := colorful.MakeColor(c.Input.At(x, y))
	if !b {
		panic(fmt.Errorf("colorful.MakeColor failed! why: %v,%v %v %v", x, y, c.Input.Bounds().Max, c.Input.At(x, y)))
	}

	buf[0], buf[1], buf[2] = temp.Hsv()
	//h, s, v :=

	//return []float64{h, s, v}
}

func (c *WarpImageConnector) Set(x, y int, data []float64) {
	clr := colorful.Hsv(data[0], data[1], data[2])
	c.Output.Set(x, y, clr)
}

func (c *WarpImageConnector) OutputDims() (int, int) {
	b := c.Output.Bounds()
	return b.Max.X, b.Max.Y
}

func (c *WarpImageConnector) NumFields() int {
	return 3
}

// -----

func GetPerspectiveTransform(src, dst []image.Point) TransformationMatrix {
	m := gocv.GetPerspectiveTransform(src, dst)
	defer m.Close()
	return togonum(&m)
}

func invert(m mat.Matrix) *mat.Dense {
	rows, cols := m.Dims()
	d := mat.NewDense(rows, cols, nil)
	err := d.Inverse(m)
	if err != nil {
		panic(err)
	}
	return d
}

func getRoundedValueHelp(input WarpConnector, r, c float64, rp, cp int, out, buf []float64) {
	dx := 1 - math.Abs(float64(rp)-r)
	dy := 1 - math.Abs(float64(cp)-c)

	area := dx * dy
	input.Get(rp, cp, buf)

	for idx, vv := range buf {
		out[idx] += vv * area
	}
}

func getRoundedValue(input WarpConnector, rows, cols int, r, c float64, total, buf []float64) []float64 {
	r0 := int(r)
	r1 := r0 + 1
	c0 := int(c)
	c1 := c0 + 1

	for idx := 0; idx < input.NumFields(); idx++ {
		total[idx] = 0
	}

	getRoundedValueHelp(input, r, c, r0, c0, total, buf)
	getRoundedValueHelp(input, r, c, r1, c0, total, buf)
	getRoundedValueHelp(input, r, c, r1, c1, total, buf)
	getRoundedValueHelp(input, r, c, r0, c1, total, buf)

	return total
}

func Warp(input WarpConnector, m TransformationMatrix) {
	m = invert(m)
	rows, cols := input.OutputDims()

	total := make([]float64, input.NumFields())
	buf := make([]float64, input.NumFields())

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {

			R := (m.At(0, 0)*float64(r) + m.At(0, 1)*float64(c) + m.At(0, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))
			C := (m.At(1, 0)*float64(r) + m.At(1, 1)*float64(c) + m.At(1, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))

			//fmt.Printf("%d %d -> %v %v\n", r, c, R, C)
			input.Set(r, c, getRoundedValue(input, rows, cols, R, C, total, buf))
		}
	}

}

func WarpImage(img image.Image, m TransformationMatrix, newSize image.Point) *image.RGBA {
	out := image.NewRGBA(image.Rectangle{image.Point{0, 0}, newSize})
	conn := &WarpImageConnector{img, out}
	Warp(conn, m)
	return conn.Output
}

func togonum(m *gocv.Mat) *mat.Dense {
	d := mat.NewDense(m.Rows(), m.Cols(), nil)
	for r := 0; r < m.Rows(); r++ {
		for c := 0; c < m.Cols(); c++ {
			d.Set(r, c, m.GetDoubleAt(r, c))
		}
	}
	return d
}

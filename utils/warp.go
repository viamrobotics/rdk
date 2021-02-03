package utils

import (
	"fmt"
	"image"
	"math"

	"gonum.org/v1/gonum/mat"

	"github.com/lucasb-eyer/go-colorful"

	"gocv.io/x/gocv"
)

type WarpConnector interface {
	Get(x, y int) []float64
	Set(x, y int, data []float64)
	Dims() (int, int)
}

// -----

type WarpMatrixConnector struct {
	Input  mat.Matrix
	Output *mat.Dense
}

func (c *WarpMatrixConnector) Get(x, y int) []float64 {
	return []float64{c.Input.At(x, y)}
}

func (c *WarpMatrixConnector) Set(x, y int, data []float64) {
	c.Output.Set(x, y, data[0])
}

func (c *WarpMatrixConnector) Dims() (int, int) {
	return c.Input.Dims()
}

// -----

type WarpImageConnector struct {
	Input  image.Image
	Output *image.RGBA
}

func (c *WarpImageConnector) Get(x, y int) []float64 {
	temp, b := colorful.MakeColor(c.Input.At(x, y))
	if !b {
		panic(fmt.Errorf("colorful.MakeColor failed! why: %v,%v %v %v", x, y, c.Input.Bounds().Max, c.Input.At(x, y)))
	}

	h, s, v := temp.Hsv()

	return []float64{h, s, v}
}

func (c *WarpImageConnector) Set(x, y int, data []float64) {
	clr := colorful.Hsv(data[0], data[1], data[2])
	c.Output.Set(x, y, clr)
}

func (c *WarpImageConnector) Dims() (int, int) {
	b := c.Input.Bounds()
	return b.Max.X, b.Max.Y
}

// -----

func GetPerspectiveTransform(src, dst []image.Point) mat.Matrix {
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

func getRoundedValueHelp(input WarpConnector, r, c float64, rp, cp int, out []float64) {
	dx := 1 - math.Abs(float64(rp)-r)
	dy := 1 - math.Abs(float64(cp)-c)

	area := dx * dy
	v := input.Get(rp, cp)

	for idx, vv := range v {
		out[idx] += vv * area
	}
}

func getRoundedValue(input WarpConnector, rows, cols int, r, c float64) []float64 {
	r0 := int(r)
	r1 := r0 + 1
	c0 := int(c)
	c1 := c0 + 1

	if r0 >= rows {
		r0 = rows - 1
	}
	if r1 >= rows {
		r1 = rows - 1
	}

	if c0 >= cols {
		c0 = cols - 1
	}
	if c1 >= cols {
		c1 = cols - 1
	}

	total := make([]float64, len(input.Get(0, 0)))

	getRoundedValueHelp(input, r, c, r0, c0, total)
	getRoundedValueHelp(input, r, c, r1, c0, total)
	getRoundedValueHelp(input, r, c, r1, c1, total)
	getRoundedValueHelp(input, r, c, r0, c1, total)

	return total
}

func Warp(input WarpConnector, m mat.Matrix) {
	m = invert(m)
	rows, cols := input.Dims()

	//out := mat.NewDense(rows, cols, nil)

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {

			R := (m.At(0, 0)*float64(r) + m.At(0, 1)*float64(c) + m.At(0, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))
			C := (m.At(1, 0)*float64(r) + m.At(1, 1)*float64(c) + m.At(1, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))

			//fmt.Printf("%d %d -> %v %v\n", r, c, R, C)
			input.Set(r, c, getRoundedValue(input, rows, cols, R, C))
		}
	}

	//return out
}

func WarpImage(img image.Image, m mat.Matrix) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
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

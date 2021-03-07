package rimage

import (
	"image"
	"math"

	"gonum.org/v1/gonum/mat"
)

type TransformationMatrix [][]float64

func (m TransformationMatrix) At(x, y int) float64 {
	return m[x][y]
}

func (m TransformationMatrix) Dims() (int, int) {
	return len(m), len(m[0])
}

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
	Input  *Image
	Output *Image
}

func (c *WarpImageConnector) Get(x, y int, buf []float64) {
	cc := c.Input.GetXY(x, y)
	cc.RawFloatArrayFill(buf)
}

func (c *WarpImageConnector) Set(x, y int, data []float64) {
	c.Output.SetXY(x, y, NewColorFromArray(data))
}

func (c *WarpImageConnector) OutputDims() (int, int) {
	b := c.Output.Bounds()
	return b.Max.X, b.Max.Y
}

func (c *WarpImageConnector) NumFields() int {
	return 6
}

// -----

// cribbed from opencv cv::getPerspectiveTransform
func GetPerspectiveTransform(src, dst []image.Point) TransformationMatrix {

	a := mat.NewDense(8, 8, nil)
	b := mat.NewDense(8, 1, nil)

	for i := 0; i < 4; i++ {
		a.Set(i+4, 3, float64(src[i].X))
		a.Set(i, 0, a.At(i+4, 3))

		a.Set(i+4, 4, float64(src[i].Y))
		a.Set(i, 1, a.At(i+4, 4))

		a.Set(i, 2, 1)
		a.Set(i+4, 5, 1)

		a.Set(i, 6, float64(-src[i].X*dst[i].X))
		a.Set(i, 7, float64(-src[i].Y*dst[i].X))
		a.Set(i+4, 6, float64(-src[i].X*dst[i].Y))
		a.Set(i+4, 7, float64(-src[i].Y*dst[i].Y))

		b.Set(i, 0, float64(dst[i].X))
		b.Set(i+4, 0, float64(dst[i].Y))
	}

	raw := make([]float64, 8)
	x := mat.NewDense(8, 1, raw)

	err := x.Solve(a, b)
	if err != nil {
		panic(err)
	}

	raw = append(raw, 1.0)
	m := mat.NewDense(3, 3, raw)

	m = invert(m)

	tm := [][]float64{
		make([]float64, 3),
		make([]float64, 3),
		make([]float64, 3),
	}

	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			tm[x][y] = m.At(x, y)
		}
	}

	return tm
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

func getRoundedValueHelp(input WarpConnector, dx, dy float64, rp, cp float64, out, buf []float64) {
	area := dx * dy
	input.Get(int(rp), int(cp), buf)

	for idx, vv := range buf {
		out[idx] += vv * area
	}
}

func getRoundedValue(input WarpConnector, r, c float64, total, buf []float64) []float64 {
	r0 := math.Floor(r)
	r1 := r0 + 1
	c0 := math.Floor(c)
	c1 := c0 + 1

	getRoundedValueHelp(input, r1-r, c1-c, r0, c0, total, buf)
	getRoundedValueHelp(input, r-r0, c1-c, r1, c0, total, buf)
	getRoundedValueHelp(input, r-r0, c-c0, r1, c1, total, buf)
	getRoundedValueHelp(input, r1-r, c-c0, r0, c1, total, buf)

	return total
}

func Warp(input WarpConnector, m TransformationMatrix) {
	rows, cols := input.OutputDims()

	numFields := input.NumFields()

	total := make([]float64, numFields)
	buf := make([]float64, numFields)

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {

			R := (m.At(0, 0)*float64(r) + m.At(0, 1)*float64(c) + m.At(0, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))
			C := (m.At(1, 0)*float64(r) + m.At(1, 1)*float64(c) + m.At(1, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))

			for idx := 0; idx < numFields; idx++ {
				total[idx] = 0
			}

			input.Set(r, c, getRoundedValue(input, R, C, total, buf))
		}
	}

}

func WarpImage(img image.Image, m TransformationMatrix, newSize image.Point) *Image {
	out := NewImage(newSize.X, newSize.Y)
	conn := &WarpImageConnector{ConvertImage(img), out}
	Warp(conn, m)
	return conn.Output
}

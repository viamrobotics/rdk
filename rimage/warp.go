//go:build !notc

package rimage

import (
	"image"
	"math"

	"github.com/pkg/errors"
	"gonum.org/v1/gonum/mat"
)

// TransformationMatrix TODO.
type TransformationMatrix [][]float64

// At TODO.
func (m TransformationMatrix) At(x, y int) float64 {
	return m[x][y]
}

// Dims TODO.
func (m TransformationMatrix) Dims() (int, int) {
	return len(m), len(m[0])
}

func newTransformationMatrix(m mat.Matrix) TransformationMatrix {
	tm := [][]float64{
		make([]float64, 3),
		make([]float64, 3),
		make([]float64, 3),
	}

	if m != nil {
		for x := 0; x < 3; x++ {
			for y := 0; y < 3; y++ {
				tm[x][y] = m.At(x, y)
			}
		}
	}

	return tm
}

// WarpConnector TODO.
type WarpConnector interface {
	// return is if the point is valid or not
	Get(x, y int, buf []float64) bool
	Set(x, y int, data []float64)
	OutputDims() (int, int)
	NumFields() int // how many float64 are in the buffers above
}

// WarpMatrixConnector TODO.
type WarpMatrixConnector struct {
	Input  mat.Matrix
	Output *mat.Dense
}

// Get TODO.
func (c *WarpMatrixConnector) Get(x, y int, buf []float64) bool {
	buf[0] = c.Input.At(x, y)
	return true
}

// Set TODO.
func (c *WarpMatrixConnector) Set(x, y int, data []float64) {
	c.Output.Set(x, y, data[0])
}

// OutputDims TODO.
func (c *WarpMatrixConnector) OutputDims() (int, int) {
	return c.Output.Dims()
}

// NumFields TODO.
func (c *WarpMatrixConnector) NumFields() int {
	return 1
}

// WarpImageConnector TODO.
type WarpImageConnector struct {
	Input  *Image
	Output *Image
}

// Get TODO.
func (c *WarpImageConnector) Get(x, y int, buf []float64) bool {
	// Note: this isn't quite correct, as we're going to averge rgb, and hsv differently.
	// I'm not sure if it matters or not, but it might
	cc := c.Input.GetXY(x, y)
	cc.RawFloatArrayFill(buf)
	return true
}

// Set TODO.
func (c *WarpImageConnector) Set(x, y int, data []float64) {
	c.Output.SetXY(x, y, NewColorFromArray(data))
}

// OutputDims TODO.
func (c *WarpImageConnector) OutputDims() (int, int) {
	b := c.Output.Bounds()
	return b.Max.X, b.Max.Y
}

// NumFields TODO.
func (c *WarpImageConnector) NumFields() int {
	return 6
}

// GetPerspectiveTransform is cribbed from opencv cv::getPerspectiveTransform.
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

	if err := x.Solve(a, b); err != nil {
		panic(err)
	}

	raw = append(raw, 1.0)
	m := mat.NewDense(3, 3, raw)

	m = invert(m)

	tm := newTransformationMatrix(m)

	return tm
}

func invert(m mat.Matrix) *mat.Dense {
	rows, cols := m.Dims()
	d := mat.NewDense(rows, cols, nil)

	// we estimate the inverse so we can solve any shape
	b := mat.NewDense(rows, cols, nil)
	b.Set(0, 0, 1)
	b.Set(1, 1, 1)
	b.Set(2, 2, 1)
	if err := d.Solve(m, b); err != nil {
		panic(errors.Wrapf(err, "cannot invert matrix %v", m))
	}
	return d
}

// returns good area.
func getRoundedValueHelp(input WarpConnector, dx, dy, rp, cp float64, out, buf []float64) float64 {
	area := dx * dy
	if area <= .00001 {
		return area
	}

	if !input.Get(int(rp), int(cp), buf) {
		// point is invalid, what do we do!!!
		return 0
	}

	for idx, vv := range buf {
		out[idx] += vv * area
	}
	return area
}

func getRoundedValue(input WarpConnector, r, c float64, total, buf []float64) []float64 {
	r0 := math.Floor(r)
	r1 := r0 + 1
	c0 := math.Floor(c)
	c1 := c0 + 1

	goodArea := 0.0

	goodArea += getRoundedValueHelp(input, r1-r, c1-c, r0, c0, total, buf)
	goodArea += getRoundedValueHelp(input, r-r0, c1-c, r1, c0, total, buf)
	goodArea += getRoundedValueHelp(input, r-r0, c-c0, r1, c1, total, buf)
	goodArea += getRoundedValueHelp(input, r1-r, c-c0, r0, c1, total, buf)

	if goodArea < .99 {
		for idx := range total {
			total[idx] /= goodArea
		}
	}

	return total
}

// Warp TODO.
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

// WarpImage TODO.
func WarpImage(img image.Image, m TransformationMatrix, newSize image.Point) *Image {
	out := NewImage(newSize.X, newSize.Y)
	conn := &WarpImageConnector{ConvertImage(img), out}
	Warp(conn, m)
	return conn.Output
}

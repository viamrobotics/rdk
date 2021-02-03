package utils

import (
	"fmt"
	"image"
	"math"

	"gonum.org/v1/gonum/mat"

	"gocv.io/x/gocv"
)

func GetPerspectiveTransform(src, dst []image.Point) mat.Matrix {
	m := gocv.GetPerspectiveTransform(src, dst)
	defer m.Close()
	return togonum(&m)
}

func invert(m mat.Matrix) *mat.Dense {
	rows, cols := m.Dims()
	d := mat.NewDense(rows, cols, nil)
	d.Inverse(m)
	return d
}

func getRoundedValueHelp(input mat.Matrix, r, c float64, rp, cp int) float64 {
	v := input.At(rp, cp)

	dx := 1 - math.Abs(float64(rp)-r)
	dy := 1 - math.Abs(float64(cp)-c)

	area := dx * dy

	//fmt.Printf("%v %v %v %v | %v\n", r, c, rp, cp, area)

	return v * area

}

func getRoundedValue(input mat.Matrix, r, c float64) float64 {
	r0 := int(r)
	r1 := r0 + 1
	c0 := int(c)
	c1 := c0 + 1

	total := 0.0
	total += getRoundedValueHelp(input, r, c, r0, c0)
	total += getRoundedValueHelp(input, r, c, r1, c0)
	total += getRoundedValueHelp(input, r, c, r1, c1)
	total += getRoundedValueHelp(input, r, c, r0, c1)

	return total
}

func Warp(input, m mat.Matrix) *mat.Dense {
	m = invert(m)
	rows, cols := input.Dims()

	out := mat.NewDense(rows, cols, nil)

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {

			R := (m.At(0, 0)*float64(r) + m.At(0, 1)*float64(c) + m.At(0, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))
			C := (m.At(1, 0)*float64(r) + m.At(1, 1)*float64(c) + m.At(1, 2)) /
				(m.At(2, 0)*float64(r) + m.At(2, 1)*float64(c) + m.At(2, 2))

			fmt.Printf("%d %d -> %v %v\n", r, c, R, C)
			out.Set(r, c, getRoundedValue(input, R, C))
		}
	}

	return out
}

func Warpgocv(input, m mat.Matrix) *mat.Dense {

	inputGocv := togocv(input)
	defer inputGocv.Close()
	mGocv := togocv(m)
	defer mGocv.Close()

	warped := gocv.NewMatWithSize(inputGocv.Rows(), inputGocv.Cols(), inputGocv.Type())
	defer warped.Close()

	gocv.WarpPerspective(inputGocv, &warped, mGocv, image.Point{inputGocv.Cols(), inputGocv.Rows()})

	return togonum(&warped)
}

func togocv(input mat.Matrix) gocv.Mat {
	rows, cols := input.Dims()
	m := gocv.NewMatWithSize(rows, cols, gocv.MatTypeCV64F)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			m.SetDoubleAt(r, c, input.At(r, c))
		}
	}
	return m
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

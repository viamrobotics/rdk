package utils

import (
	"fmt"
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"gonum.org/v1/gonum/mat"
)

func increasingArray(start, inc float64, total int) []float64 {
	data := make([]float64, total)
	for i := 0; i < total; i++ {
		data[i] = start + float64(i)*inc
	}
	return data
}

func TestGetRoundedValue(t *testing.T) {
	m := mat.NewDense(2, 2, []float64{0, 1, 10, 20})

	assert.Equal(t, 0.0, getRoundedValue(m, 0, 0))
	assert.Equal(t, 0.5, getRoundedValue(m, 0, 0.5))
}

func TestWarp1(t *testing.T) {
	size := 5

	m := GetPerspectiveTransform(
		[]image.Point{
			{1, 1},
			{size - 1, 1},
			{1, size - 1},
			{size - 1, size - 1},
		},
		[]image.Point{
			{0, 0},
			{size, 0},
			{0, size},
			{size, size},
		})

	fmt.Println(m)

	input := mat.NewDense(size, size, increasingArray(0, 1, size*size))

	res1 := Warpgocv(input, m)
	fmt.Println(res1)

	res2 := Warp(input, m)
	fmt.Println(res2)

}

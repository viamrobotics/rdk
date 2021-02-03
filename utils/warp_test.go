package utils

import (
	"image"
	"os"
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

	input := mat.NewDense(size, size, increasingArray(0, 1, size*size))

	res := mat.NewDense(size, size, nil)
	Warp(&WarpMatrixConnector{input, res}, m)

	assert.Equal(t, 6.0, res.At(0, 0))
	assert.Equal(t, 20.4, res.At(4, 4))
}

func TestWarp2(t *testing.T) {
	img, err := ReadImageFromFile("data/canny1.png")
	if err != nil {
		t.Fatal(err)
	}

	size := 800

	m := GetPerspectiveTransform(
		[]image.Point{
			{100, 100},
			{700, 100},
			{100, 700},
			{700, 700},
		},
		[]image.Point{
			{0, 0},
			{size, 0},
			{0, size},
			{size, size},
		})

	out := WarpImage(img, m, image.Point{size, size})

	os.MkdirAll("out", 0775)
	err = WriteImageToFile("out/canny1-warped.png", out)
	if err != nil {
		t.Fatal(err)
	}

}

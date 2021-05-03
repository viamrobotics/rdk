package rimage

import (
	"image"
	"testing"

	"go.viam.com/robotcore/artifact"

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

	src := []image.Point{
		{1, 1},
		{size - 1, 1},
		{1, size - 1},
		{size - 1, size - 1},
	}
	dst := []image.Point{
		{0, 0},
		{size, 0},
		{0, size},
		{size, size},
	}

	m2 := GetPerspectiveTransform(src, dst)
	r, c := m2.Dims()
	assert.Equal(t, 3, r, c)
	assert.Equal(t, 3, r, c)
	assert.InEpsilon(t, 0.5999999999999999, m2.At(0, 0), .01)

	input := mat.NewDense(size, size, increasingArray(0, 1, size*size))

	res := mat.NewDense(size, size, nil)
	Warp(&WarpMatrixConnector{input, res}, m2)

	assert.InEpsilon(t, 6.0, res.At(0, 0), .01)
	assert.InEpsilon(t, 20.4, res.At(4, 4), .01)
}

func TestWarp2(t *testing.T) {
	img, err := NewImageFromFile(artifact.MustPath("rimage/canny1.png"))
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

	err = WriteImageToFile(outDir+"/canny1-warped.png", out)
	if err != nil {
		t.Fatal(err)
	}

}

func BenchmarkWarp(b *testing.B) {
	img, err := NewImageFromFile(artifact.MustPath("rimage/canny1.png"))
	if err != nil {
		b.Fatal(err)
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

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		WarpImage(img, m, image.Point{size, size})
	}
}

func TestWarpInvert(t *testing.T) {

	toSlice := func(m mat.Matrix) []float64 {
		a := []float64{}
		for x := 0; x < 3; x++ {
			for y := 0; y < 3; y++ {
				a = append(a, m.At(x, y))
			}
		}
		return a
	}

	doTest := func(inSlice, correct []float64) {
		input := mat.NewDense(3, 3, inSlice)
		output := invert(input)
		assert.InDeltaSlice(t, correct, toSlice(output), .01)

	}

	doTest(
		[]float64{1.66, 0, -1.66, 0, 1.66, -1.66, 0, 0, 1},
		[]float64{0.6, 0, 1, 0, 0.6, 1.0, 0, 0, 1},
	)

	doTest(
		[]float64{1.3333333333333333, 0, -133.3333, 0, 1.3333, -133.333, -0, -0, 1},
		[]float64{0.75, 0, 100, 0, 0.75, 100, 0, 0, 1},
	)
}

func TestWarpSmall1(t *testing.T) {
	// this is mostly making sure this test actually runs
	// as it requires a non-standard matrix invert
	img, err := readImageFromFile(artifact.MustPath("rimage/warpsmall1.jpg"), false)
	if err != nil {
		t.Fatal(err)
	}

	outputSize := image.Point{100, 100}
	x := WarpImage(img, GetPerspectiveTransform(
		[]image.Point{{0, 170}, {0, 0}, {223, 0}, {223, 170}},
		ArrayToPoints([]image.Point{{0, 0}, {outputSize.X - 1, outputSize.Y - 1}}),
	), outputSize)

	err = WriteImageToFile(outDir+"/warpsmall1.png", x)
	if err != nil {
		t.Fatal(err)
	}

}

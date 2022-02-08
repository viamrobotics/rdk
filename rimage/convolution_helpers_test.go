package rimage

import (
	"image"
	"image/color"
	"testing"

	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
)

func TestPaddingFloat64(t *testing.T) {
	img := mat.NewDense(10, 10, nil)
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			img.Set(i, j, float64(i*i+j*j))
		}
	}
	nRows, nCols := img.Dims()
	test.That(t, nRows, test.ShouldEqual, 10)
	test.That(t, nCols, test.ShouldEqual, 10)
	padded1, err := PaddingFloat64(img, image.Point{3, 3}, image.Point{1, 1}, BorderConstant)
	test.That(t, err, test.ShouldBeNil)
	paddedHeight1, paddedWidth1 := padded1.Dims()
	test.That(t, paddedHeight1, test.ShouldEqual, 12)
	test.That(t, paddedWidth1, test.ShouldEqual, 12)
	test.That(t, padded1.At(0, 0), test.ShouldEqual, 0.)
	test.That(t, padded1.At(11, 11), test.ShouldEqual, 0.)
}

func TestPaddingGray(t *testing.T) {
	rect := image.Rect(0, 0, 10, 12)
	img := image.NewGray(rect)
	for x := 0; x < 10; x++ {
		for y := 0; y < 12; y++ {
			img.Set(x, y, color.Gray{uint8(x*x + y*y)})
		}
	}
	// testing constant padding
	paddedConstant, err := PaddingGray(img, image.Point{3, 3}, image.Point{1, 1}, BorderConstant)
	test.That(t, err, test.ShouldBeNil)
	rect2 := paddedConstant.Bounds()
	test.That(t, rect2.Max.X, test.ShouldEqual, 12)
	test.That(t, rect2.Max.Y, test.ShouldEqual, 14)
	test.That(t, paddedConstant.At(0, 0), test.ShouldResemble, color.Gray{0})
	test.That(t, paddedConstant.At(11, 13), test.ShouldResemble, color.Gray{0})
	// testing reflect padding
	paddedReflect, err := PaddingGray(img, image.Point{3, 3}, image.Point{1, 1}, BorderReflect)
	test.That(t, err, test.ShouldBeNil)
	rect3 := paddedConstant.Bounds()
	test.That(t, rect3.Max.X, test.ShouldEqual, 12)
	test.That(t, rect3.Max.Y, test.ShouldEqual, 14)
	test.That(t, paddedReflect.At(0, 0), test.ShouldResemble, color.Gray{2})
	test.That(t, paddedReflect.At(11, 13), test.ShouldResemble, color.Gray{164})
	// testing replicate padding
	paddedReplicate, err := PaddingGray(img, image.Point{3, 3}, image.Point{1, 1}, BorderReplicate)
	test.That(t, err, test.ShouldBeNil)
	rect4 := paddedConstant.Bounds()
	test.That(t, rect4.Max.X, test.ShouldEqual, 12)
	test.That(t, rect4.Max.Y, test.ShouldEqual, 14)
	test.That(t, paddedReplicate.At(0, 0), test.ShouldResemble, color.Gray{1})
	test.That(t, paddedReplicate.At(11, 13), test.ShouldResemble, color.Gray{202})
}

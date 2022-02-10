package rimage

import (
	"image"
	"math/rand"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestConvolveGray(t *testing.T) {
	// test that image test files are in artifacts
	im, err := NewImageFromFile(artifact.MustPath("rimage/binary_image.jpg"))
	test.That(t, err, test.ShouldBeNil)
	gt, err := NewImageFromFile(artifact.MustPath("rimage/sobelx.png"))
	test.That(t, err, test.ShouldBeNil)
	// Create a new grayscale image
	bounds := im.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y
	imGray := image.NewGray(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			imGray.Set(x, y, im.At(x, y))
		}
	}
	// Create a new grayscale image for GT
	boundsGT := im.Bounds()
	wGT, hGT := boundsGT.Max.X, boundsGT.Max.Y
	imGTGray := image.NewGray(image.Rect(0, 0, wGT, hGT))
	for x := 0; x < wGT; x++ {
		for y := 0; y < hGT; y++ {
			imGTGray.Set(x, y, gt.At(x, y))
		}
	}
	// convolve image
	kernel := GetSobelX()
	convolved, err := ConvolveGray(imGray, &kernel, image.Point{1, 1}, 0)
	test.That(t, err, test.ShouldBeNil)
	// check size
	test.That(t, convolved.Rect.Max.X, test.ShouldEqual, w)
	test.That(t, convolved.Rect.Max.Y, test.ShouldEqual, h)
	// compare 2 non zero pixel values
	test.That(t, convolved.At(97, 47), test.ShouldResemble, imGTGray.At(97, 47))
	test.That(t, convolved.At(536, 304), test.ShouldResemble, imGTGray.At(536, 304))
}

func TestConvolveGrayFloat64(t *testing.T) {
	// test that image test files are in artifacts
	im, err := NewImageFromFile(artifact.MustPath("rimage/binary_image.jpg"))
	bounds := im.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y
	test.That(t, err, test.ShouldBeNil)
	// convert to gray float
	imGray := ConvertColorImageToLuminanceFloat(im)
	x, y := rand.Intn(w), rand.Intn(h)
	r, g, b, _ := im.GetXY(x, y).RGBA()
	test.That(t, imGray.At(y, x), test.ShouldEqual, float64(r))
	test.That(t, imGray.At(y, x), test.ShouldEqual, float64(g))
	test.That(t, imGray.At(y, x), test.ShouldEqual, float64(b))
	// load gt image
	gt, err := NewImageFromFile(artifact.MustPath("rimage/sobelx.png"))
	test.That(t, err, test.ShouldBeNil)
	// convert to gray float
	gtGray := ConvertColorImageToLuminanceFloat(gt)
	x1, y1 := rand.Intn(w), rand.Intn(h)
	r1, g1, b1, _ := gt.GetXY(x1, y1).RGBA()
	test.That(t, gtGray.At(y1, x1), test.ShouldEqual, float64(r1))
	test.That(t, gtGray.At(y1, x1), test.ShouldEqual, float64(g1))
	test.That(t, gtGray.At(y1, x1), test.ShouldEqual, float64(b1))

	kernel := GetSobelX()
	convolved, err := ConvolveGrayFloat64(imGray, &kernel)
	test.That(t, err, test.ShouldBeNil)
	// check size
	nRows, nCols := convolved.Dims()
	test.That(t, nCols, test.ShouldEqual, w)
	test.That(t, nRows, test.ShouldEqual, h)
	// compare 2 non zero pixel values
	test.That(t, convolved.At(47, 97), test.ShouldResemble, gtGray.At(47, 97)) // 0 < val < 255
	test.That(t, convolved.At(304, 536), test.ShouldEqual, -1)                 // val < 0 - not clamped
}

func TestGetSobelY(t *testing.T) {
	k := GetSobelY()
	test.That(t, k.Height, test.ShouldEqual, 3)
	test.That(t, k.Width, test.ShouldEqual, 3)
	test.That(t, k.At(0, 0), test.ShouldEqual, -1)
	test.That(t, k.At(0, 1), test.ShouldEqual, -2)
	test.That(t, k.At(0, 2), test.ShouldEqual, -1)
	test.That(t, k.At(1, 0), test.ShouldEqual, 0)
}

func TestGetBlur3(t *testing.T) {
	k := GetBlur3()
	test.That(t, k.Height, test.ShouldEqual, 3)
	test.That(t, k.Width, test.ShouldEqual, 3)
	test.That(t, k.At(0, 0), test.ShouldEqual, 1)
	test.That(t, k.At(0, 1), test.ShouldEqual, 1)
	test.That(t, k.At(0, 2), test.ShouldEqual, 1)
	test.That(t, k.At(1, 0), test.ShouldEqual, 1)
}

func TestGetGaussian3(t *testing.T) {
	k := GetGaussian3()
	test.That(t, k.Height, test.ShouldEqual, 3)
	test.That(t, k.Width, test.ShouldEqual, 3)
	test.That(t, k.At(0, 0), test.ShouldEqual, 1)
	test.That(t, k.At(0, 1), test.ShouldEqual, 2)
	test.That(t, k.At(0, 2), test.ShouldEqual, 1)
	test.That(t, k.At(1, 0), test.ShouldEqual, 2)
	normalized := (&k).Normalize()
	test.That(t, normalized.Height, test.ShouldEqual, 3)
	test.That(t, normalized.Width, test.ShouldEqual, 3)
	test.That(t, normalized.At(0, 0), test.ShouldEqual, 1./16.)
	test.That(t, normalized.At(0, 1), test.ShouldEqual, 2./16.)
	test.That(t, normalized.At(0, 2), test.ShouldEqual, 1./16.)
	test.That(t, normalized.At(1, 0), test.ShouldEqual, 2./16.)
}

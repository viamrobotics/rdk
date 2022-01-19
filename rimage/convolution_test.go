package rimage

import (
	"image"
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

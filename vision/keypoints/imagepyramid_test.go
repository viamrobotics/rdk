package keypoints

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"go.viam.com/test"
)

func TestDownscaleImage(t *testing.T) {
	// create test image
	rectImage := image.NewGray(image.Rect(0, 0, 300, 200))
	whiteRect := image.Rect(50, 30, 100, 150)
	white := color.Gray{255}
	black := color.Gray{0}
	draw.Draw(rectImage, rectImage.Bounds(), &image.Uniform{black}, image.Point{0, 0}, draw.Src)
	draw.Draw(rectImage, whiteRect, &image.Uniform{white}, image.Point{0, 0}, draw.Src)

	// downsize image
	downsized, err := downscaleNearestGrayImage(rectImage, 2.)
	// test no error
	test.That(t, err, test.ShouldBeNil)
	// test new size
	newSize := downsized.Bounds().Max
	test.That(t, newSize.X, test.ShouldEqual, rectImage.Rect.Max.X/2)
	test.That(t, newSize.Y, test.ShouldEqual, rectImage.Rect.Max.Y/2)
	// test values around white rect border in downscaled image
	test.That(t, downsized.At(25, 15).(color.Gray).Y, test.ShouldEqual, 255)
	test.That(t, downsized.At(49, 74).(color.Gray).Y, test.ShouldEqual, 255)
	test.That(t, downsized.At(24, 15).(color.Gray).Y, test.ShouldEqual, 0)
	test.That(t, downsized.At(50, 75).(color.Gray).Y, test.ShouldEqual, 0)
}

func TestGetNumberOctaves(t *testing.T) {
	tests := []struct {
		imgSize image.Point
		want    int
	}{
		{image.Point{200, 400}, 7},
		{image.Point{16, 8}, 2},
		{image.Point{2, 8}, 0},
	}
	for _, tst := range tests {
		nOct := GetNumberOctaves(tst.imgSize)
		test.That(t, nOct, test.ShouldEqual, tst.want)
	}
}

func TestGetImagePyramid(t *testing.T) {
	// create test image
	rectImage := image.NewGray(image.Rect(0, 0, 300, 200))
	whiteRect := image.Rect(50, 30, 100, 150)
	white := color.Gray{255}
	black := color.Gray{0}
	draw.Draw(rectImage, rectImage.Bounds(), &image.Uniform{black}, image.Point{0, 0}, draw.Src)
	draw.Draw(rectImage, whiteRect, &image.Uniform{white}, image.Point{0, 0}, draw.Src)

	pyramid, err := GetImagePyramid(rectImage)
	// test no error
	test.That(t, err, test.ShouldBeNil)
	// test number scales / octaves
	test.That(t, len(pyramid.Images), test.ShouldEqual, 7)
	test.That(t, len(pyramid.Scales), test.ShouldEqual, 7)
	test.That(t, len(pyramid.Scales), test.ShouldEqual, len(pyramid.Scales))
	// test image sizes
	imgSize1 := pyramid.Images[1].Bounds().Max
	test.That(t, imgSize1.X, test.ShouldEqual, rectImage.Rect.Max.X/2)
	test.That(t, imgSize1.Y, test.ShouldEqual, rectImage.Rect.Max.Y/2)
	imgSize2 := pyramid.Images[2].Bounds().Max
	test.That(t, imgSize2.X, test.ShouldEqual, rectImage.Rect.Max.X/4)
	test.That(t, imgSize2.Y, test.ShouldEqual, rectImage.Rect.Max.Y/4)
}

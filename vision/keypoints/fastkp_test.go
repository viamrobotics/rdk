package keypoints

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func createTestImage() *image.Gray {
	rectImage := image.NewGray(image.Rect(0, 0, 300, 200))
	whiteRect := image.Rect(50, 30, 100, 150)
	white := color.Gray{255}
	black := color.Gray{0}
	draw.Draw(rectImage, rectImage.Bounds(), &image.Uniform{black}, image.Point{0, 0}, draw.Src)
	draw.Draw(rectImage, whiteRect, &image.Uniform{white}, image.Point{0, 0}, draw.Src)
	return rectImage
}

func TestLoadFASTConfiguration(t *testing.T) {
	cfg, err := LoadFASTConfiguration("kpconfig.json")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg.Threshold, test.ShouldEqual, 0.15)
	test.That(t, cfg.NMatchesCircle, test.ShouldEqual, 9)
	test.That(t, cfg.NMSWinSize, test.ShouldEqual, 7)
}

func TestGetPointValuesInNeighborhood(t *testing.T) {
	// create test image
	rectImage := createTestImage()
	// testing cross neighborhood
	vals := GetPointValuesInNeighborhood(rectImage, image.Point{50, 30}, CrossIdx)
	// test length
	test.That(t, len(vals), test.ShouldEqual, 4)
	// test values at a corner of the rectangle
	test.That(t, vals[0], test.ShouldEqual, 255)
	test.That(t, vals[1], test.ShouldEqual, 255)
	test.That(t, vals[2], test.ShouldEqual, 0)
	test.That(t, vals[3], test.ShouldEqual, 0)
	// testing circle neighborhood
	valsCircle := GetPointValuesInNeighborhood(rectImage, image.Point{50, 30}, CircleIdx)
	// test length
	test.That(t, len(valsCircle), test.ShouldEqual, 16)
	// test values at a corner of the rectangle
	test.That(t, valsCircle[0], test.ShouldEqual, 0)
	test.That(t, valsCircle[1], test.ShouldEqual, 0)
	test.That(t, valsCircle[2], test.ShouldEqual, 0)
	test.That(t, valsCircle[3], test.ShouldEqual, 0)
	test.That(t, valsCircle[4], test.ShouldEqual, 255)
	test.That(t, valsCircle[5], test.ShouldEqual, 255)
	test.That(t, valsCircle[6], test.ShouldEqual, 255)
	test.That(t, valsCircle[7], test.ShouldEqual, 255)
	test.That(t, valsCircle[8], test.ShouldEqual, 255)
	for i := 9; i < len(valsCircle); i++ {
		test.That(t, valsCircle[i], test.ShouldEqual, 0)
	}
}

func TestComputeFAST(t *testing.T) {
	// first test should return no keypoints
	// create test image
	rectImage := createTestImage()
	cfg, err := LoadFASTConfiguration("kpconfig.json")
	test.That(t, err, test.ShouldBeNil)
	kps, err := ComputeFAST(rectImage, cfg.NMatchesCircle, cfg.NMSWinSize, cfg.Threshold)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(kps), test.ShouldEqual, 0)

	// test on a chess image
	// test that image test files are in artifacts
	im, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess3.jpg"))
	test.That(t, err, test.ShouldBeNil)
	// Convert to grayscale image
	bounds := im.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y
	imGray := image.NewGray(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			imGray.Set(x, y, im.At(x, y))
		}
	}
	// compute kps
	kpsChess, err := ComputeFAST(imGray, cfg.NMatchesCircle, cfg.NMSWinSize, cfg.Threshold)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(kpsChess), test.ShouldEqual, 107)
	err = PlotKeypoints(imGray, kpsChess, "/tmp/keypoints.png")
	test.That(t, err, test.ShouldBeNil)
}

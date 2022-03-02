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
	cfg := LoadFASTConfiguration("kpconfig.json")
	test.That(t, cfg, test.ShouldNotBeNil)
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

func TestIsValidSlice(t *testing.T) {
	tests := []struct {
		s        []float64
		n        int
		expected bool
	}{
		{[]float64{0, 0, 0, 0, 0}, 9, false},
		{[]float64{1, 1, 1, 1, 1, 1, 1}, 3, true},
		{[]float64{0, 1, 1, 1, 0, 1, 1}, 2, true},
		{[]float64{0, 1, 1, 0, 0, 1, 0}, 2, false},
	}
	for _, tst := range tests {
		test.That(t, isValidSliceVals(tst.s, tst.n), test.ShouldEqual, tst.expected)
	}
}

func TestSumPositiveValues(t *testing.T) {
	tests := []struct {
		s        []float64
		expected float64
	}{
		{[]float64{0, 0, 0, 0, 0}, 0},
		{[]float64{1, -1, -1, 0, 1, 1, 1}, 4},
		{[]float64{-1, -1, -1, 0, -1, -1, -1}, 0},
	}
	for _, tst := range tests {
		test.That(t, sumOfPositiveValuesSlice(tst.s), test.ShouldEqual, tst.expected)
	}
}

func TestSumNegativeValues(t *testing.T) {
	tests := []struct {
		s        []float64
		expected float64
	}{
		{[]float64{0, 0, 0, 0, 0}, 0},
		{[]float64{1, -1, -1, 0, 1, 1, 1}, -2},
		{[]float64{-1, -1, -1, 0, -1, -1, -1}, -6},
	}
	for _, tst := range tests {
		test.That(t, sumOfNegativeValuesSlice(tst.s), test.ShouldEqual, tst.expected)
	}
}

func TestGetBrighterValues(t *testing.T) {
	tests := []struct {
		s        []float64
		t        float64
		expected []float64
	}{
		{[]float64{1, 10, 3, 1, 20, 11}, 10, []float64{0, 0, 0, 0, 1, 1}},
		{[]float64{1, 1, 1, 1}, 1, []float64{0, 0, 0, 0}},
	}
	for _, tst := range tests {
		test.That(t, getBrighterValues(tst.s, tst.t), test.ShouldResemble, tst.expected)
	}
}

func TestGetDarkerValues(t *testing.T) {
	tests := []struct {
		s        []float64
		t        float64
		expected []float64
	}{
		{[]float64{1, 10, 3, 1, 20, 11}, 10, []float64{1, 0, 1, 1, 0, 0}},
		{[]float64{1, 1, 1, 1}, 1, []float64{0, 0, 0, 0}},
	}
	for _, tst := range tests {
		test.That(t, getDarkerValues(tst.s, tst.t), test.ShouldResemble, tst.expected)
	}
}

func TestComputeFAST(t *testing.T) {
	// load config
	cfg := LoadFASTConfiguration("kpconfig.json")
	test.That(t, cfg, test.ShouldNotBeNil)
	// load image from artifacts and convert to gray image
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
	kpsChess := ComputeFAST(imGray, cfg)
	test.That(t, len(kpsChess), test.ShouldEqual, 100)
	err = PlotKeypoints(imGray, kpsChess, "/tmp/keypoints2.png")
	test.That(t, err, test.ShouldBeNil)
	// test with rectangle image
	rectImage := createTestImage()
	kps := ComputeFAST(rectImage, cfg)
	test.That(t, len(kps), test.ShouldEqual, 2)
	test.That(t, kps[0], test.ShouldResemble, image.Point{50, 149})
	test.That(t, kps[1], test.ShouldResemble, image.Point{99, 149})
}

func TestNewFASTKeypointsFromImage(t *testing.T) {
	// load config
	cfg := LoadFASTConfiguration("kpconfig.json")
	test.That(t, cfg, test.ShouldNotBeNil)
	// load image from artifacts and convert to gray image
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
	fastKps := NewFASTKeypointsFromImage(imGray, cfg)
	test.That(t, len(fastKps.Points), test.ShouldEqual, 100)
	test.That(t, len(fastKps.Orientations), test.ShouldEqual, 100)
	// value from opencv FAST orientation computation
	test.That(t, fastKps.Orientations[0], test.ShouldAlmostEqual, 0.058798250129)
	isOriented1 := fastKps.IsOriented()
	test.That(t, isOriented1, test.ShouldBeTrue)

	// test no orientation
	cfg.Oriented = false
	fastKpsNoOrientation := NewFASTKeypointsFromImage(imGray, cfg)
	test.That(t, len(fastKpsNoOrientation.Points), test.ShouldEqual, 100)
	test.That(t, fastKpsNoOrientation.Orientations, test.ShouldBeNil)
	isOriented2 := fastKpsNoOrientation.IsOriented()
	test.That(t, isOriented2, test.ShouldBeFalse)
}

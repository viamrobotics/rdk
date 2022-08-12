package keypoints

import (
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestComputeBRIEFDescriptors(t *testing.T) {
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

	// load BRIEF cfg
	cfgBrief := LoadBRIEFConfiguration("brief.json")
	test.That(t, cfgBrief, test.ShouldNotBeNil)
	briefDescriptors, err := ComputeBRIEFDescriptors(imGray, fastKps, cfgBrief)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(briefDescriptors), test.ShouldEqual, len(fastKps.Points))

	// // test that for any descriptor (random index in descriptor), all values are either 0 or 1 (square of value is equal to value)
	// randIdx := rand.Intn(len(briefDescriptors))
	// for _, desc := range briefDescriptors[randIdx] {
	// 	test.That(t, desc*desc, test.ShouldEqual, desc)
	// }
}

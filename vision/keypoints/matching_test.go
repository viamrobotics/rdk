package keypoints

import (
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestRangeInt(t *testing.T) {
	u1, l1 := 2, -5
	step1 := 1
	r1 := rangeInt(u1, l1, step1)
	test.That(t, len(r1), test.ShouldEqual, 7)
	test.That(t, r1[0], test.ShouldEqual, -5)
	test.That(t, r1[6], test.ShouldEqual, 1)

	u2, l2 := 8, 2
	step2 := 2
	r2 := rangeInt(u2, l2, step2)
	test.That(t, len(r2), test.ShouldEqual, 3)
	test.That(t, r2[0], test.ShouldEqual, 2)
	test.That(t, r2[1], test.ShouldEqual, 4)
	test.That(t, r2[2], test.ShouldEqual, 6)
}

func TestMatchKeypoints(t *testing.T) {
	// load config
	cfg := LoadFASTConfiguration("kpconfig.json")
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

	// load BRIEF cfg
	cfgBrief := LoadBRIEFConfiguration("brief.json")
	briefDescriptors, err := ComputeBRIEFDescriptors(imGray, fastKps, cfgBrief)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(briefDescriptors), test.ShouldEqual, len(fastKps.Points))

	// image 2
	// load image from artifacts and convert to gray image
	im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess.jpg"))
	test.That(t, err, test.ShouldBeNil)
	// Convert to grayscale image
	bounds2 := im2.Bounds()
	w2, h2 := bounds2.Max.X, bounds2.Max.Y
	imGray2 := image.NewGray(image.Rect(0, 0, w2, h2))
	for x := 0; x < w2; x++ {
		for y := 0; y < h2; y++ {
			imGray2.Set(x, y, im2.At(x, y))
		}
	}
	fastKps2 := NewFASTKeypointsFromImage(imGray2, cfg)

	// load BRIEF cfg
	briefDescriptors2, err := ComputeBRIEFDescriptors(imGray2, fastKps2, cfgBrief)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(briefDescriptors2), test.ShouldEqual, len(fastKps2.Points))
	// matches
	cfgMatch := MatchingConfig{
		false,
		1000,
	}
	// test matches with itself
	matches := MatchKeypoints(briefDescriptors, briefDescriptors, cfgMatch)
	for _, match := range matches.Indices {
		test.That(t, match.Idx1, test.ShouldEqual, match.Idx2)
	}
	// test matches with bigger image
	cfgMatch.DoCrossCheck = true
	matches2 := MatchKeypoints(briefDescriptors, briefDescriptors2, cfgMatch)
	test.That(t, len(matches2.Indices), test.ShouldEqual, 14)
}

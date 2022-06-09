package keypoints

import (
	"image"
	"image/draw"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestLoadORBConfiguration(t *testing.T) {
	cfg := LoadORBConfiguration("orbconfig.json")
	test.That(t, cfg, test.ShouldNotBeNil)
	test.That(t, cfg.Layers, test.ShouldEqual, 4)
	test.That(t, cfg.DownscaleFactor, test.ShouldEqual, 2)
	test.That(t, cfg.FastConf.Threshold, test.ShouldEqual, 0.15)
	test.That(t, cfg.FastConf.NMatchesCircle, test.ShouldEqual, 9)
	test.That(t, cfg.FastConf.NMSWinSize, test.ShouldEqual, 7)
}

func TestComputeORBKeypoints(t *testing.T) {
	cfg := LoadORBConfiguration("orbconfig.json")
	test.That(t, cfg, test.ShouldNotBeNil)
	// load image from artifacts and convert to gray image
	im, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess3.jpg"))
	test.That(t, err, test.ShouldBeNil)
	// Convert to grayscale image
	bounds := im.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y
	imGray := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(imGray, imGray.Bounds(), im, im.Bounds().Min, draw.Src)
	descs, kps, err := ComputeORBKeypoints(imGray, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(descs), test.ShouldEqual, 137)
	test.That(t, len(kps), test.ShouldEqual, 137)
}

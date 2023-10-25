package keypoints

import (
	"image"
	"image/draw"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
)

func TestLoadORBConfiguration(t *testing.T) {
	cfg, err := LoadORBConfiguration("orbconfig.json")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg, test.ShouldNotBeNil)
	test.That(t, cfg.Layers, test.ShouldEqual, 4)
	test.That(t, cfg.DownscaleFactor, test.ShouldEqual, 2)
	test.That(t, cfg.FastConf.Threshold, test.ShouldEqual, 20)
	test.That(t, cfg.FastConf.NMatchesCircle, test.ShouldEqual, 9)
	test.That(t, cfg.FastConf.NMSWinSize, test.ShouldEqual, 7)

	// test config validation
	cfg1 := &ORBConfig{
		Layers: 0,
	}
	err = cfg1.Validate("")
	test.That(t, err, test.ShouldBeError)
	test.That(t, err.Error(), test.ShouldEqual, "error validating \"\": n_layers should be >= 1")

	cfg2 := &ORBConfig{
		Layers:          2,
		DownscaleFactor: 1,
	}
	err = cfg2.Validate("")
	test.That(t, err, test.ShouldBeError)
	test.That(t, err.Error(), test.ShouldEqual, "error validating \"\": downscale_factor should be greater than 1")

	cfg3 := &ORBConfig{
		Layers:          4,
		DownscaleFactor: 2,
	}
	err = cfg3.Validate("")
	test.That(t, err, test.ShouldBeError)
}

func TestComputeORBKeypoints(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := LoadORBConfiguration("orbconfig.json")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg, test.ShouldNotBeNil)
	// load image from artifacts and convert to gray image
	im, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess3.jpg"))
	test.That(t, err, test.ShouldBeNil)
	// Convert to grayscale image
	bounds := im.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y
	imGray := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(imGray, imGray.Bounds(), im, im.Bounds().Min, draw.Src)
	samplePoints := GenerateSamplePairs(cfg.BRIEFConf.Sampling, cfg.BRIEFConf.N, cfg.BRIEFConf.PatchSize)
	descs, kps, err := ComputeORBKeypoints(imGray, samplePoints, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(descs), test.ShouldEqual, 58)
	test.That(t, len(kps), test.ShouldEqual, 58)
	// save the output image in a temp file
	tempDir := t.TempDir()
	logger.Infof("writing orb keypoint files to %s", tempDir)
	keyImg := PlotKeypoints(imGray, kps)
	test.That(t, keyImg, test.ShouldNotBeNil)
	err = rimage.WriteImageToFile(tempDir+"/orb_keypoints.png", keyImg)
	test.That(t, err, test.ShouldBeNil)
}

func TestMatchingWithRotation(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg, err := LoadORBConfiguration("orbconfig.json")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg, test.ShouldNotBeNil)
	// load images from artifacts and convert to gray image
	im, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess3_rotate.jpg"))
	test.That(t, err, test.ShouldBeNil)
	imGray := rimage.MakeGray(im)
	imBig, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess.jpg"))
	test.That(t, err, test.ShouldBeNil)
	imBigGray := rimage.MakeGray(imBig)
	// compute orb points for each image
	samplePoints := GenerateSamplePairs(cfg.BRIEFConf.Sampling, cfg.BRIEFConf.N, cfg.BRIEFConf.PatchSize)
	orb1, kps1, err := ComputeORBKeypoints(imGray, samplePoints, cfg)
	test.That(t, err, test.ShouldBeNil)
	orb2, kps2, err := ComputeORBKeypoints(imBigGray, samplePoints, cfg)
	test.That(t, err, test.ShouldBeNil)
	cfgMatch := &MatchingConfig{DoCrossCheck: true, MaxDist: 400}
	matches := MatchDescriptors(orb1, orb2, cfgMatch, logger)
	matchedKps1, matchedKps2, err := GetMatchingKeyPoints(matches, kps1, kps2)
	test.That(t, err, test.ShouldBeNil)
	matchedOrbPts1 := PlotKeypoints(imGray, matchedKps1)
	matchedOrbPts2 := PlotKeypoints(imBigGray, matchedKps2)
	matchedLines := PlotMatchedLines(matchedOrbPts1, matchedOrbPts2, matchedKps1, matchedKps2, true)
	test.That(t, matchedLines, test.ShouldNotBeNil)
	// save the output image in a temp file
	tempDir := t.TempDir()
	logger.Infof("writing orb keypoint files to %s", tempDir)
	err = rimage.WriteImageToFile(tempDir+"/rotated_chess_orb.png", matchedLines)
	test.That(t, err, test.ShouldBeNil)
}

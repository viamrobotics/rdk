package keypoints

import (
	"image"
	"image/draw"
	"math"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
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

	// test u < l
	u3, l3 := 2, 8
	step3 := 2
	r3 := rangeInt(u3, l3, step3)
	test.That(t, len(r3), test.ShouldEqual, 3)
	test.That(t, r3[0], test.ShouldEqual, 2)
	test.That(t, r3[1], test.ShouldEqual, 4)
	test.That(t, r3[2], test.ShouldEqual, 6)
}

func TestMatchDescriptors(t *testing.T) {
	logger := logging.NewTestLogger(t)
	tempDir := t.TempDir()

	logger.Infof("writing sample points to %s", tempDir)
	// load config
	cfg := LoadFASTConfiguration("kpconfig.json")
	// load image from artifacts and convert to gray image
	im, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess3.jpg"))
	test.That(t, err, test.ShouldBeNil)
	// Convert to grayscale image
	imGray := rimage.MakeGray(im)
	fastKps := NewFASTKeypointsFromImage(imGray, cfg)
	t.Logf("number of keypoints in img 1: %d", len(fastKps.Points))
	keyPtsImg := PlotKeypoints(imGray, fastKps.Points)
	err = rimage.WriteImageToFile(tempDir+"/chessKps_1.png", keyPtsImg)
	test.That(t, err, test.ShouldBeNil)

	// image 2
	// load image from artifacts and convert to gray image
	im2, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess.jpg"))
	test.That(t, err, test.ShouldBeNil)
	// Convert to grayscale image
	imGray2 := rimage.MakeGray(im2)
	fastKps2 := NewFASTKeypointsFromImage(imGray2, cfg)
	t.Logf("number of keypoints in img 2: %d", len(fastKps2.Points))
	keyPtsImg2 := PlotKeypoints(imGray2, fastKps2.Points)
	err = rimage.WriteImageToFile(tempDir+"/chessKps_2.png", keyPtsImg2)
	test.That(t, err, test.ShouldBeNil)

	// load BRIEF cfg
	cfgBrief := LoadBRIEFConfiguration("brief.json")
	samplePoints := GenerateSamplePairs(cfgBrief.Sampling, cfgBrief.N, cfgBrief.PatchSize)

	briefDescriptors, err := ComputeBRIEFDescriptors(imGray, samplePoints, fastKps, cfgBrief)
	t.Logf("number of descriptors in img 1: %d", len(briefDescriptors))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(briefDescriptors), test.ShouldEqual, len(fastKps.Points))

	briefDescriptors2, err := ComputeBRIEFDescriptors(imGray2, samplePoints, fastKps2, cfgBrief)
	t.Logf("number of descriptors in img 2: %d", len(briefDescriptors2))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(briefDescriptors2), test.ShouldEqual, len(fastKps2.Points))
	// matches
	cfgMatch := MatchingConfig{
		true,
		400,
	}
	// test matches with itself
	matches := MatchDescriptors(briefDescriptors, briefDescriptors, &cfgMatch, logger)
	t.Logf("number of matches in img 1: %d", len(matches))
	matchedKps1, matchedKps2, err := GetMatchingKeyPoints(matches, fastKps.Points, fastKps.Points)
	test.That(t, err, test.ShouldBeNil)
	matchedLinesImg := PlotMatchedLines(imGray, imGray, matchedKps1, matchedKps2, false)
	err = rimage.WriteImageToFile(tempDir+"/matched_chess.png", matchedLinesImg)
	test.That(t, err, test.ShouldBeNil)
	for _, match := range matches {
		test.That(t, match.Idx1, test.ShouldEqual, match.Idx2)
	}
	// test matches with bigger image and cross-check; #matches <= #kps2
	matches = MatchDescriptors(briefDescriptors, briefDescriptors2, &cfgMatch, logger)
	t.Logf("number of matches in img 1 vs img 2: %d", len(matches))
	test.That(t, len(matches), test.ShouldBeLessThanOrEqualTo, len(fastKps2.Points))
	matchedKps1, matchedKps2, err = GetMatchingKeyPoints(matches, fastKps.Points, fastKps2.Points)
	test.That(t, err, test.ShouldBeNil)
	matchedLinesImg = PlotMatchedLines(imGray, imGray2, matchedKps1, matchedKps2, false)
	err = rimage.WriteImageToFile(tempDir+"/bigger_matched_chess.png", matchedLinesImg)
	test.That(t, err, test.ShouldBeNil)
}

func TestGetMatchingKeyPoints(t *testing.T) {
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

	// get matches
	cfgMatch := MatchingConfig{
		true,
		1000,
	}
	// test matches with itself
	matches := MatchDescriptors(descs, descs, &cfgMatch, logger)

	kps1, kps2, err := GetMatchingKeyPoints(matches, kps, kps)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(kps1), test.ShouldEqual, len(kps2))
	for i, pt1 := range kps1 {
		pt2 := kps2[i]
		test.That(t, math.Abs(float64(pt1.X-pt2.X)), test.ShouldBeLessThan, 1)
		test.That(t, math.Abs(float64(pt1.Y-pt2.Y)), test.ShouldBeLessThan, 1)
	}
}

func TestOrbMatching(t *testing.T) {
	logger := logging.NewTestLogger(t)
	orbConf := &ORBConfig{
		Layers:          4,
		DownscaleFactor: 2,
		FastConf: &FASTConfig{
			NMatchesCircle: 9,
			NMSWinSize:     7,
			Threshold:      20,
			Oriented:       true,
			Radius:         16,
		},
		BRIEFConf: &BRIEFConfig{
			N:              512,
			Sampling:       2,
			UseOrientation: true,
			PatchSize:      48,
		},
	}
	matchingConf := &MatchingConfig{
		DoCrossCheck: true,
		MaxDist:      1000,
	}
	img1, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000001.png"))
	test.That(t, err, test.ShouldBeNil)
	img2, err := rimage.NewImageFromFile(artifact.MustPath("vision/odometry/000002.png"))
	test.That(t, err, test.ShouldBeNil)
	im1 := rimage.MakeGray(img1)
	im2 := rimage.MakeGray(img2)
	samplePoints := GenerateSamplePairs(orbConf.BRIEFConf.Sampling, orbConf.BRIEFConf.N, orbConf.BRIEFConf.PatchSize)
	// image 1
	orb1, _, err := ComputeORBKeypoints(im1, samplePoints, orbConf)
	test.That(t, err, test.ShouldBeNil)
	// image 2
	orb2, _, err := ComputeORBKeypoints(im2, samplePoints, orbConf)
	test.That(t, err, test.ShouldBeNil)
	matches := MatchDescriptors(orb1, orb2, matchingConf, logger)
	test.That(t, len(matches), test.ShouldBeGreaterThan, 300)
	test.That(t, len(matches), test.ShouldBeLessThan, 350)
}

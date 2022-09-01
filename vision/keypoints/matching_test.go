package keypoints

import (
	"image"
	"image/draw"
	"math"
	"sort"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
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

	// test u < l
	u3, l3 := 2, 8
	step3 := 2
	r3 := rangeInt(u3, l3, step3)
	test.That(t, len(r3), test.ShouldEqual, 3)
	test.That(t, r3[0], test.ShouldEqual, 2)
	test.That(t, r3[1], test.ShouldEqual, 4)
	test.That(t, r3[2], test.ShouldEqual, 6)
}

func TestMatchKeypoints(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// load config
	cfg := LoadFASTConfiguration("kpconfig.json")
	// load image from artifacts and convert to gray image
	im, err := rimage.NewImageFromFile(artifact.MustPath("vision/keypoints/chess3.jpg"))
	test.That(t, err, test.ShouldBeNil)
	// Convert to grayscale image
	bounds := im.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y
	imGray := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(imGray, imGray.Bounds(), im, im.Bounds().Min, draw.Src)
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
	draw.Draw(imGray2, imGray2.Bounds(), im2, im2.Bounds().Min, draw.Src)
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
	matches := MatchKeypoints(briefDescriptors, briefDescriptors, &cfgMatch, logger)
	for _, match := range matches.Indices {
		test.That(t, match.Idx1, test.ShouldEqual, match.Idx2)
	}
	// test matches with bigger image
	matches2 := MatchKeypoints(briefDescriptors, briefDescriptors2, &cfgMatch, logger)
	test.That(t, len(matches2.Indices), test.ShouldEqual, len(matches2.Descriptors1))
	// test matches with bigger image and cross-check; #matches <= #kps2
	cfgMatch.DoCrossCheck = true
	matches3 := MatchKeypoints(briefDescriptors, briefDescriptors2, &cfgMatch, logger)
	test.That(t, len(matches3.Indices), test.ShouldBeLessThanOrEqualTo, len(fastKps2.Points))
}

func TestGetMatchingKeyPoints(t *testing.T) {
	logger := golog.NewTestLogger(t)
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
	descs, kps, err := ComputeORBKeypoints(imGray, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(descs), test.ShouldEqual, 137)
	test.That(t, len(kps), test.ShouldEqual, 137)

	// get matches
	cfgMatch := MatchingConfig{
		true,
		1000,
	}
	// test matches with itself
	matches := MatchKeypoints(descs, descs, &cfgMatch, logger)

	kps1, kps2, err := GetMatchingKeyPoints(matches, kps, kps)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(kps1), test.ShouldEqual, len(kps2))
	for i, pt1 := range kps1 {
		pt2 := kps2[i]
		// test.That(t, pt1, test.ShouldResemble, pt2)
		test.That(t, math.Abs(float64(pt1.X-pt2.X)), test.ShouldBeLessThan, 1)
		test.That(t, math.Abs(float64(pt1.Y-pt2.Y)), test.ShouldBeLessThan, 1)
	}
}

func TestOrbMatching(t *testing.T) {
	logger := golog.NewTestLogger(t)
	orbConf := &ORBConfig{
		Layers:          4,
		DownscaleFactor: 2,
		FastConf: &FASTConfig{
			NMatchesCircle: 9,
			NMSWinSize:     7,
			Threshold:      .10,
			Oriented:       true,
			Radius:         16,
		},
		BRIEFConf: &BRIEFConfig{
			N:              256,
			Sampling:       2,
			UseOrientation: true,
			PatchSize:      16,
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
	// image 1
	orb1, kps1, err := ComputeORBKeypoints(im1, orbConf)
	test.That(t, err, test.ShouldBeNil)
	orb1, err = sortDescriptorsByPoint(orb1, kps1)
	test.That(t, err, test.ShouldBeNil)
	// image 2
	orb2, kps2, err := ComputeORBKeypoints(im2, orbConf)
	test.That(t, err, test.ShouldBeNil)
	orb2, err = sortDescriptorsByPoint(orb2, kps2)
	test.That(t, err, test.ShouldBeNil)
	matches := MatchKeypoints(orb1, orb2, matchingConf, logger)
	test.That(t, len(matches.Indices), test.ShouldBeGreaterThan, 300)
	test.That(t, len(matches.Indices), test.ShouldBeLessThan, 350)
}

func sortDescriptorsByPoint(desc Descriptors, kps KeyPoints) (Descriptors, error) {
	if len(desc) != len(kps) {
		return nil, errors.Errorf("number of descriptors (%d) does not equal number of keypoints (%d)", len(desc), len(kps))
	}
	// sort by point order
	type ptdesc struct {
		Kp  image.Point
		Des Descriptor
	}

	sorted := make([]ptdesc, 0, len(kps))
	for i := range kps {
		sorted = append(sorted, ptdesc{kps[i], desc[i]})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Kp.X > sorted[j].Kp.X
	})
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Kp.Y > sorted[j].Kp.Y
	})
	sortedDesc := make(Descriptors, 0, len(desc))
	for i := range sorted {
		sortedDesc = append(sortedDesc, sorted[i].Des)
	}
	return sortedDesc, nil
}

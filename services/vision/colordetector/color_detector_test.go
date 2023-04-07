package colordetector

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestColorDetector(t *testing.T) {
	inp := objectdetection.ColorDetectorConfig{
		SegmentSize:       150000,
		HueTolerance:      0.44,
		DetectColorString: "#4F3815",
	}
	ctx := context.Background()
	r := &inject.Robot{}
	testlog := golog.NewLogger("testlog")
	srv, err := registerColorDetector(ctx, "test_cd", &inp, r, testlog)
	test.That(t, err, test.ShouldBeNil)
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	det, err := srv.Detections(ctx, img, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, det, test.ShouldHaveLength, 1)
	_, err = srv.Classifications(ctx, img, 1, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")

	// with error - bad parameters
	inp.HueTolerance = 4.0 // value out of range
	_, err = registerColorDetector(ctx, "test_cd", &inp, r, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "hue_tolerance_pct must be between")

	// with error - nil parameters
	_, err = registerColorDetector(ctx, "test_cd", nil, r, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")
}

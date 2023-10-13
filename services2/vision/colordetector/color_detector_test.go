package colordetector

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/objectdetection"
)

func TestColorDetector(t *testing.T) {
	inp := objectdetection.ColorDetectorConfig{
		SegmentSize:       150000,
		HueTolerance:      0.44,
		DetectColorString: "#4F3815",
	}
	ctx := context.Background()
	r := &inject.Robot{}
	name := vision.Named("test_cd")
	srv, err := registerColorDetector(ctx, name, &inp, r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, srv.Name(), test.ShouldResemble, name)
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	// Does implement Detections
	det, err := srv.Detections(ctx, img, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, det, test.ShouldHaveLength, 1)

	// Does not implement Classifications
	_, err = srv.Classifications(ctx, img, 1, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")

	// with error - bad parameters
	inp.HueTolerance = 4.0 // value out of range
	_, err = registerColorDetector(ctx, name, &inp, r)
	test.That(t, err.Error(), test.ShouldContainSubstring, "hue_tolerance_pct must be between")

	// with error - nil parameters
	_, err = registerColorDetector(ctx, name, nil, r)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")
}

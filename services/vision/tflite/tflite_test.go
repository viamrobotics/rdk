package tflite

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

func TestNewTfLiteDetector(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	ctx := context.Background()
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg := &TFLiteDetectorConfig{
		ModelPath:  modelLoc,
		LabelPath:  nil,
		NumThreads: 1,
	}
	r := &inject.Robot{}
	got2, err := registerTFLiteDetector(ctx, "test_tf", cfg, r, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	gotDetections, err := got2.Detections(ctx, pic, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.789)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.7)

	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "17")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "17")
	test.That(t, goutils.TryClose(ctx, got2), test.ShouldBeNil)

}

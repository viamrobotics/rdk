package tflite

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/testutils/inject"
)

func TestNewTfLiteDetector(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	ctx := context.Background()
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg := &DetectorConfig{
		ModelPath:  modelLoc,
		LabelPath:  "",
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

func TestNewTfLiteClassifier(t *testing.T) {
	ctx := context.Background()
	// Test that a classifier would give an expected output on the lion image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	modelLoc := artifact.MustPath("vision/tflite/effnet0.tflite")
	cfg := &ClassifierConfig{
		ModelPath:  modelLoc,
		LabelPath:  "",
		NumThreads: 1,
	}
	r := &inject.Robot{}
	got2, err := registerTFLiteClassifier(ctx, "tf_class", cfg, r, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got2, test.ShouldNotBeNil)

	outClass, err := got2.Classifications(ctx, pic, 5, nil)
	test.That(t, err, test.ShouldBeNil)
	bestOut, err := outClass.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bestOut[0].Label(), test.ShouldResemble, "291")
	test.That(t, bestOut[0].Score(), test.ShouldBeGreaterThan, 0.82)
	test.That(t, goutils.TryClose(ctx, got2), test.ShouldBeNil)
}

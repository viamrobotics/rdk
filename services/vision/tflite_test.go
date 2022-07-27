package vision

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
)

func BenchmarkAddTFLiteDetector(b *testing.B) {
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg := DetectorConfig{
		Name: "testdetector", Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}
	ctx := context.Background()
	logger := golog.NewLogger("benchmark")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := NewTFLiteDetector(ctx, &cfg, logger)
		test.That(b, err, test.ShouldBeNil)
	}
}

func BenchmarkGetTFLiteDetections(b *testing.B) {
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(b, err, test.ShouldBeNil)
	cfg := DetectorConfig{
		Name: "testdetector", Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}
	ctx := context.Background()
	logger := golog.NewLogger("benchmark")
	det, model, err := NewTFLiteDetector(ctx, &cfg, logger)
	test.That(b, model, test.ShouldNotBeNil)
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detections, err := det(ctx, pic)
		test.That(b, detections, test.ShouldNotBeNil)
		test.That(b, err, test.ShouldBeNil)
	}
}

func TestNewTfLiteDetector(t *testing.T) {
	// Test that empty config gives error about loading model
	emptyCfg := DetectorConfig{}
	ctx := context.Background()
	got, model, err := NewTFLiteDetector(ctx, &emptyCfg, golog.NewTestLogger(t))
	test.That(t, model, test.ShouldBeNil)
	test.That(t, got, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "something wrong with adding the model")

	// Test that a detector would give an expected output on the dog image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg := DetectorConfig{
		Name: "testdetector", Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 1,
		},
	}

	got2, model, err := NewTFLiteDetector(ctx, &cfg, golog.NewTestLogger(t))
	test.That(t, model, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	gotDetections, err := got2(ctx, pic)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.789)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.7)

	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "17")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "17")

	test.That(t, err, test.ShouldBeNil)
}

func TestMoreDetectorModels(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)

	// Build SSD detector
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/ssdmobilenet.tflite")
	cfg := DetectorConfig{
		Name: "testssddetector", Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}
	outSSD, outSSDModel, err := NewTFLiteDetector(ctx, &cfg, golog.NewTestLogger(t))
	test.That(t, outSSDModel, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	// Test that SSD detector output is as expected on image
	got, err := outSSD(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got[0].Label(), test.ShouldResemble, "17")
	test.That(t, got[1].Label(), test.ShouldResemble, "17")
	test.That(t, got[0].Score(), test.ShouldBeGreaterThan, 0.82)
	test.That(t, got[1].Score(), test.ShouldBeGreaterThan, 0.8)

	modelLoc = artifact.MustPath("vision/tflite/mobilenet.tflite")
	cfg = DetectorConfig{
		Name: "mobilenetdetector", Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}

	outMNet, outMNetModel, err := NewTFLiteDetector(ctx, &cfg, golog.NewTestLogger(t))
	test.That(t, outMNetModel, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	got2, err := outMNet(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got2[0].Label(), test.ShouldResemble, "0")
	test.That(t, got2[1].Label(), test.ShouldResemble, "0")
	test.That(t, got2[0].Score(), test.ShouldBeGreaterThan, 0.89)
	test.That(t, got2[1].Score(), test.ShouldBeGreaterThan, 0.89)
}

func TestLabelReader(t *testing.T) {
	inputfile := artifact.MustPath("vision/tflite/fakelabels.txt")
	got, err := loadLabels(inputfile)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got[0], test.ShouldResemble, "this")
	test.That(t, len(got), test.ShouldEqual, 12)
}

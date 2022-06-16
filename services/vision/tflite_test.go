package vision

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
)

func TestNewTfLiteDetector(t *testing.T) {
	// Test that empty config gives error about loading model
	emptyCfg := DetectorConfig{}
	got, err := NewTfliteDetector(&emptyCfg, golog.NewTestLogger(t))
	test.That(t, got, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "something wrong with adding the model")

	// Test that a detector would give an expected output on the dog image
	pic, _ := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg := DetectorConfig{
		Name: "testdetector", Type: "tflite",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 1,
		},
	}

	got2, err := NewTfliteDetector(&cfg, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	gotDetections, err := got2(pic)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.8)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.7)

	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "17")
	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "17")

	test.That(t, err, test.ShouldBeNil)
}

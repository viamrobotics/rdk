package mlvision

import (
	"context"
	"github.com/nfnt/resize"
	"go.viam.com/rdk/services/mlmodel/tflitecpu"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestNewMLDetector(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	// Set it up as a ML Model

	ctx := context.Background()
	// r := &inject.Robot{}
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	labelLoc := artifact.MustPath("vision/tflite/effdetlabels.txt")
	// name := "myMLDet"
	cfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  &labelLoc,
	}
	// visCfg := MLModelConfig{ModelName: "myMLDet"}
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	// Test that a detector would give the expected output on the dog image
	// Creating the model should populate model and attrs, but not metadata
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, "myMLDet")
	check, err := out.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, check.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, check.Outputs[0].Name, test.ShouldResemble, "location")
	test.That(t, check.Outputs[1].Name, test.ShouldResemble, "category")
	test.That(t, check.Outputs[1].Extra["labels"], test.ShouldNotBeNil)

	resized := resize.Resize(320, 320, pic, resize.Bilinear)

	inMap := make(map[string]interface{})
	inMap["image"] = rimage.ImageToUInt8Buffer(resized)
	gotOut, err := out.Infer(ctx, inMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOut, test.ShouldNotBeNil)

	gotDetector, err := attemptToBuildDetector(out)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err := gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.789)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.7)
	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "Dog")

}

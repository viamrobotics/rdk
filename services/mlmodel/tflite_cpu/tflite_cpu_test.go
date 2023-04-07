package tflitecpu

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestNewTFLiteCPUModel(t *testing.T) {
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")

	emptyCfg := TFLiteConfig{}
	cfg := TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 1,
	}

	// Test that empty config gives error about loading model
	emptyGot, err := CreateTFLiteCPUModel(ctx, &emptyCfg)
	test.That(t, emptyGot, test.ShouldResemble, &TFLiteCPUModel{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "could not add model")
	test.That(t, cfg, test.ShouldNotBeNil)

	// Test that a detector would give an expected output on the dog image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	imgBytes := rimage.ImageToUInt8Buffer(pic)
	test.That(t, imgBytes, test.ShouldNotBeNil)

	got, err := CreateTFLiteCPUModel(ctx, &cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.model, test.ShouldNotBeNil)
	test.That(t, got.attrs, test.ShouldNotBeNil)
	test.That(t, got.metadata, test.ShouldBeNil)

	// Test that the Metadata() works
	gotMD, err := got.Metadata(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotMD, test.ShouldNotBeNil)

	test.That(t, gotMD.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, gotMD.Outputs[0].Name, test.ShouldResemble, "location")
	test.That(t, gotMD.Outputs[1].Name, test.ShouldResemble, "category")
	test.That(t, gotMD.Outputs[2].Name, test.ShouldResemble, "score")
	test.That(t, gotMD.Outputs[1].AssociatedFiles[0].Name, test.ShouldResemble, "labelmap.txt")

	// Test that the Infer() works
	inputMap := make(map[string]interface{})
	inputMap["image"] = imgBytes
	gotOutput, err := got.Infer(ctx, inputMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOutput, test.ShouldNotBeNil)

	test.That(t, gotOutput["number of detections"], test.ShouldResemble, []float32{25})
	test.That(t, len(gotOutput["score"].([]float32)), test.ShouldResemble, 25)
	test.That(t, len(gotOutput["location"].([]float32)), test.ShouldResemble, 100)
	test.That(t, len(gotOutput["category"].([]float32)), test.ShouldResemble, 25)

	// TODO: Khari. Make sure that the values are actually CORRECT, not just there.
	// To also do: Test the tflite classifier and make sure it's vibing too
}

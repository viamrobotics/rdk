package tflitecpu

import (
	"context"
	"math/rand"
	"testing"

	"github.com/nfnt/resize"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
)

func TestNewTFLiteCPUModel(t *testing.T) {
	// Setup
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	modelLoc2 := artifact.MustPath("vision/tflite/effnet0.tflite")
	modelLoc3 := artifact.MustPath("vision/tflite/mobileBERT.tflite")
	emptyCfg := TFLiteConfig{} // empty config
	cfg := TFLiteConfig{       // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	cfg2 := TFLiteConfig{ // classifier config
		ModelPath:  modelLoc2,
		NumThreads: 2,
	}
	cfg3 := TFLiteConfig{ // text classifier config
		ModelPath:  modelLoc3,
		NumThreads: 1,
	}

	// Test that empty config gives error about loading model
	emptyGot, err := CreateTFLiteCPUModel(ctx, &emptyCfg)
	test.That(t, emptyGot, test.ShouldResemble, &Model{})
	test.That(t, err.Error(), test.ShouldContainSubstring, "could not add model")
	test.That(t, cfg, test.ShouldNotBeNil)

	// Test that a detector would give the expected output on the dog image
	// Creating the model should populate model and attrs, but not metadata
	got, err := CreateTFLiteCPUModel(ctx, &cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.model, test.ShouldNotBeNil)
	test.That(t, got.attrs, test.ShouldNotBeNil)
	test.That(t, got.metadata, test.ShouldBeNil)

	// Test that the Metadata() works on detector
	gotMD, err := got.Metadata(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotMD, test.ShouldNotBeNil)
	test.That(t, got.metadata, test.ShouldNotBeNil)

	test.That(t, gotMD.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, gotMD.Outputs[0].Name, test.ShouldResemble, "location")
	test.That(t, gotMD.Outputs[1].Name, test.ShouldResemble, "category")
	test.That(t, gotMD.Outputs[2].Name, test.ShouldResemble, "score")
	test.That(t, gotMD.Inputs[0].DataType, test.ShouldResemble, "uint8")
	test.That(t, gotMD.Outputs[0].DataType, test.ShouldResemble, "float32")
	test.That(t, gotMD.Outputs[1].AssociatedFiles[0].Name, test.ShouldResemble, "labelmap.txt")

	// Test that the Infer() works on detector
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	resized := resize.Resize(uint(got.metadata.Inputs[0].Shape[1]), uint(got.metadata.Inputs[0].Shape[2]), pic, resize.Bilinear)
	imgBytes := rimage.ImageToUInt8Buffer(resized)
	test.That(t, imgBytes, test.ShouldNotBeNil)
	inputMap := make(map[string]interface{})
	inputMap["image"] = imgBytes

	gotOutput, err := got.Infer(ctx, inputMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOutput, test.ShouldNotBeNil)

	test.That(t, gotOutput["number of detections"], test.ShouldResemble, []float32{25})
	test.That(t, len(gotOutput["score"].([]float32)), test.ShouldResemble, 25)
	test.That(t, len(gotOutput["location"].([]float32)), test.ShouldResemble, 100)
	test.That(t, len(gotOutput["category"].([]float32)), test.ShouldResemble, 25)
	test.That(t, gotOutput["category"].([]float32)[0], test.ShouldEqual, 17) // 17 is dog

	// Test that the tflite classifier gives the expected output on the lion image
	got2, err := CreateTFLiteCPUModel(ctx, &cfg2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got2.model, test.ShouldNotBeNil)
	test.That(t, got2.attrs, test.ShouldNotBeNil)
	test.That(t, got2.metadata, test.ShouldBeNil)

	// Test that the Metadata() works on classifier
	gotMD2, err := got2.Metadata(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotMD2, test.ShouldNotBeNil)

	test.That(t, gotMD2.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, gotMD2.Outputs[0].Name, test.ShouldResemble, "probability")
	test.That(t, gotMD2.Inputs[0].DataType, test.ShouldResemble, "uint8")
	test.That(t, gotMD2.Outputs[0].DataType, test.ShouldResemble, "uint8")
	test.That(t, gotMD2.Outputs[0].AssociatedFiles[0].Name, test.ShouldContainSubstring, ".txt")
	test.That(t, gotMD2.Outputs[0].AssociatedFiles[0].LabelType, test.ShouldResemble, mlmodel.LabelTypeTensorAxis)

	// Test that the Infer() works on a classifier
	pic2, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	resized2 := resize.Resize(uint(got2.metadata.Inputs[0].Shape[1]), uint(got2.metadata.Inputs[0].Shape[2]), pic2, resize.Bilinear)
	imgBytes2 := rimage.ImageToUInt8Buffer(resized2)
	test.That(t, imgBytes2, test.ShouldNotBeNil)
	inputMap2 := make(map[string]interface{})
	inputMap2["image"] = imgBytes2

	gotOutput2, err := got2.Infer(ctx, inputMap2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOutput2, test.ShouldNotBeNil)

	test.That(t, gotOutput2["probability"].([]uint8), test.ShouldNotBeNil)
	test.That(t, gotOutput2["probability"].([]uint8)[290], test.ShouldEqual, 0)
	test.That(t, gotOutput2["probability"].([]uint8)[291], test.ShouldBeGreaterThan, 200) // 291 is lion
	test.That(t, gotOutput2["probability"].([]uint8)[292], test.ShouldEqual, 0)

	// Test that even a text classifier gives an output with good input
	got3, err := CreateTFLiteCPUModel(ctx, &cfg3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got3.model, test.ShouldNotBeNil)
	test.That(t, got3.attrs, test.ShouldNotBeNil)
	test.That(t, got3.metadata, test.ShouldBeNil)

	// Test that the Metadata() errors well when metadata does not exist
	_, err = got3.Metadata(ctx)
	test.That(t, err.Error(), test.ShouldContainSubstring, "metadata does not exist")
	test.That(t, got3.metadata, test.ShouldBeNil)

	// Test that the Infer() works even on a text classifier
	inputMap3 := make(map[string]interface{})
	inputMap3["text"] = makeRandomSlice(got3.model.Info.InputHeight) // text model takes
	test.That(t, len(inputMap3["text"].([]int32)), test.ShouldEqual, 384)
	gotOutput3, err := got3.Infer(ctx, inputMap3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotOutput3, test.ShouldNotBeNil)

	test.That(t, len(gotOutput3), test.ShouldEqual, 2)
	test.That(t, gotOutput3["output0"], test.ShouldNotBeNil)
	test.That(t, gotOutput3["output1"], test.ShouldNotBeNil)
	test.That(t, gotOutput3["output2"], test.ShouldBeNil)

	test.That(t, len(gotOutput3["output0"].([]float32)), test.ShouldEqual, 384)
	test.That(t, len(gotOutput3["output1"].([]float32)), test.ShouldEqual, 384)
}

func makeRandomSlice(length int) []int32 {
	out := make([]int32, 0, length)
	for i := 0; i < length; i++ {
		x := rand.Int31n(100)
		out = append(out, x)
	}
	return out
}

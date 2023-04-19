package mlvision

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel/tflitecpu"
)

func TestNewMLDetector(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	// Set it up as a ML Model

	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	labelLoc := artifact.MustPath("vision/tflite/effdetlabels.txt")
	cfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  &labelLoc,
	}
	noLabelCfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
	}

	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	// Test that a detector would give the expected output on the dog image
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, "myMLDet")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	check, err := out.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, check.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, check.Outputs[0].Name, test.ShouldResemble, "location")
	test.That(t, check.Outputs[1].Name, test.ShouldResemble, "category")
	test.That(t, check.Outputs[1].Extra["labels"], test.ShouldNotBeNil)

	gotDetector, err := attemptToBuildDetector(out)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err := gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.789)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.7)
	test.That(t, gotDetections[0].BoundingBox().Min.X, test.ShouldEqual, 126)
	test.That(t, gotDetections[0].BoundingBox().Min.Y, test.ShouldEqual, 42)
	test.That(t, gotDetections[0].BoundingBox().Max.X, test.ShouldEqual, 199)
	test.That(t, gotDetections[0].BoundingBox().Max.Y, test.ShouldEqual, 162)
	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "Dog")

	// Ensure that the same model without labelpath responds similarly
	outNL, err := tflitecpu.NewTFLiteCPUModel(ctx, &noLabelCfg, "myOtherMLDet")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outNL, test.ShouldNotBeNil)
	gotDetectorNL, err := attemptToBuildDetector(outNL)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectorNL, test.ShouldNotBeNil)
	gotDetectionsNL, err := gotDetectorNL(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectionsNL[0].Score(), test.ShouldBeGreaterThan, 0.789)
	test.That(t, gotDetectionsNL[1].Score(), test.ShouldBeGreaterThan, 0.7)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.X, test.ShouldEqual, 126)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.Y, test.ShouldEqual, 42)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.X, test.ShouldEqual, 199)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.Y, test.ShouldEqual, 162)
	test.That(t, gotDetectionsNL[0].Label(), test.ShouldResemble, "17")
	test.That(t, gotDetectionsNL[1].Label(), test.ShouldResemble, "17")
}

func TestNewMLClassifier(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	// Set it up as a ML Model

	ctx := context.Background()
	// r := &inject.Robot{}
	modelLoc := artifact.MustPath("vision/tflite/effnet0.tflite")
	labelLoc := artifact.MustPath("vision/tflite/imagenetlabels.txt")
	// name := "myMLDet"
	cfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  &labelLoc,
	}
	noLabelCfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	// Test that a classifier would give the expected result on the lion image
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, "myMLClassif")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	check, err := out.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, check.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, check.Outputs[0].Name, test.ShouldResemble, "probability")
	test.That(t, check.Outputs[0].Extra["labels"], test.ShouldNotBeNil)

	gotClassifier, err := attemptToBuildClassifier(out)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifier, test.ShouldNotBeNil)

	gotClassifications, err := gotClassifier(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifications, test.ShouldNotBeNil)
	gotTop, err := gotClassifications.TopN(5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotTop, test.ShouldNotBeNil)
	test.That(t, gotTop[0].Label(), test.ShouldContainSubstring, "lion")
	test.That(t, gotTop[0].Score(), test.ShouldBeGreaterThan, 0.99)
	test.That(t, gotTop[1].Score(), test.ShouldBeLessThan, 0.01)

	// Ensure that the same model without labelpath responds similarly
	outNL, err := tflitecpu.NewTFLiteCPUModel(ctx, &noLabelCfg, "myOtherMLClassif")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outNL, test.ShouldNotBeNil)
	gotClassifierNL, err := attemptToBuildClassifier(outNL)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifierNL, test.ShouldNotBeNil)
	gotClassificationsNL, err := gotClassifierNL(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassificationsNL, test.ShouldNotBeNil)
	topNL, err := gotClassificationsNL.TopN(5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, topNL, test.ShouldNotBeNil)
	test.That(t, topNL[0].Label(), test.ShouldContainSubstring, "291")
	test.That(t, topNL[0].Score(), test.ShouldBeGreaterThan, 0.99)
	test.That(t, topNL[1].Score(), test.ShouldBeLessThan, 0.01)
}

package builtin

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/nfnt/resize"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
)

func BenchmarkAddTFLiteDetector(b *testing.B) {
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg := vision.VisModelConfig{
		Name: "testdetector", Type: "tflite_detector",
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
	cfg := vision.VisModelConfig{
		Name: "testdetector", Type: "tflite_detector",
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
	emptyCfg := vision.VisModelConfig{}
	ctx := context.Background()
	got, model, err := NewTFLiteDetector(ctx, &emptyCfg, golog.NewTestLogger(t))
	test.That(t, model, test.ShouldBeNil)
	test.That(t, got, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "something wrong with adding the model")

	// Test that a detector would give an expected output on the dog image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	cfg := vision.VisModelConfig{
		Name: "testdetector", Type: "tflite_detector",
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
	cfg := vision.VisModelConfig{
		Name: "testssddetector", Type: "tflite_detector",
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
	cfg = vision.VisModelConfig{
		Name: "mobilenetdetector", Type: "tflite_detector",
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

func TestFileNotFound(t *testing.T) {

	// Build SSD detector
	ctx := context.Background()

	cfg := vision.VisModelConfig{
		Name: "nofile", Type: "tflite_detector",
		Parameters: config.AttributeMap{
			"model_path":  "very/fake/path.tflite",
			"label_path":  "",
			"num_threads": 2,
		},
	}
	outDet, outModel, err := NewTFLiteDetector(ctx, &cfg, golog.NewTestLogger(t))
	test.That(t, outDet, test.ShouldBeNil)
	test.That(t, outModel, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "file not found")

}

func TestNewTfLiteClassifier(t *testing.T) {
	// Test that empty config gives error about loading model
	emptyCfg := vision.VisModelConfig{}
	ctx := context.Background()
	got, model, err := NewTFLiteClassifier(ctx, &emptyCfg, golog.NewTestLogger(t))
	test.That(t, model, test.ShouldBeNil)
	test.That(t, got, test.ShouldBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "something wrong with adding the model")

	// Test that a classifier would give an expected output on the lion image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	modelLoc := artifact.MustPath("vision/tflite/effnet0.tflite")
	cfg := vision.VisModelConfig{
		Name: "testclassifier", Type: "tflite_classifier",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}

	got2, model, err := NewTFLiteClassifier(ctx, &cfg, golog.NewTestLogger(t))
	test.That(t, model, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	outClass, err := got2(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	bestOut, err := outClass.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bestOut[0].Label(), test.ShouldResemble, "291")
	test.That(t, bestOut[0].Score(), test.ShouldBeGreaterThan, 0.82)
}

func TestMoreClassifierModels(t *testing.T) {
	ctx := context.Background()

	// Test that a classifier would give an expected output on the redpanda image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/redpanda.jpeg"))
	test.That(t, err, test.ShouldBeNil)

	modelLoc := artifact.MustPath("vision/tflite/mobilenetv2_class.tflite")
	cfg := vision.VisModelConfig{
		Name: "testclassifier", Type: "tflite_classifier",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}
	got, _, err := NewTFLiteClassifier(ctx, &cfg, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	classifications, err := got(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	bestClass, err := classifications.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bestClass[0].Label(), test.ShouldResemble, "390")
	test.That(t, bestClass[0].Score(), test.ShouldBeGreaterThan, 0.93)

	// Test that a classifier would give an expected output on the redpanda image
	pic, err = rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)

	modelLoc = artifact.MustPath("vision/tflite/mobilenetv2_imagenet.tflite")
	cfg = vision.VisModelConfig{
		Name: "testclassifier", Type: "tflite_classifier",
		Parameters: config.AttributeMap{
			"model_path":  modelLoc,
			"label_path":  "",
			"num_threads": 2,
		},
	}
	got2, _, err := NewTFLiteClassifier(ctx, &cfg, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	classifications, err = got2(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	bestClass, err = classifications.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bestClass[0].Label(), test.ShouldResemble, "292")
	test.That(t, bestClass[0].Score(), test.ShouldBeGreaterThan, 0.93)
}

func TestInvalidLabels(t *testing.T) {
	ctx := context.Background()

	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/redpanda.jpeg"))
	test.That(t, err, test.ShouldBeNil)

	modelLoc := artifact.MustPath("vision/tflite/mobilenetv2_class.tflite")
	labelPath := artifact.MustPath("vision/classification/object_labels.txt")
	numThreads := 2

	labels, err := loadLabels(labelPath)
	model, err := addTFLiteModel(ctx, modelLoc, &numThreads)
	resizedImg := resize.Resize(100, 100, pic, resize.Bilinear)
	outTensor, err := tfliteInfer(ctx, model, resizedImg)

	classifications, err := unpackClassificationTensor(ctx, outTensor, model, labels)
	test.That(t, err, test.ShouldResemble, LABEL_OUTPUT_MISMATCH)
	test.That(t, classifications, test.ShouldBeNil)
}

func TestSpaceDelineatedLabels(t *testing.T) {
	labelPath := artifact.MustPath("vision/classification/lorem.txt")

	labels, err := loadLabels(labelPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(labels), test.ShouldEqual, 10)
}

//go:build !no_tflite

package mlvision

import (
	"context"
	"sync"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/mlmodel/tflitecpu"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/classification"
)

func BenchmarkAddMLVisionModel(b *testing.B) {
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")

	name := mlmodel.Named("myMLModel")
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	ctx := context.Background()
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, name)
	test.That(b, err, test.ShouldBeNil)
	test.That(b, out, test.ShouldNotBeNil)
	modelCfg := MLModelConfig{ModelName: name.Name}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service, err := registerMLModelVisionService(ctx, name, &modelCfg, &inject.Robot{}, logging.NewLogger("benchmark"))
		test.That(b, err, test.ShouldBeNil)
		test.That(b, service, test.ShouldNotBeNil)
		test.That(b, service.Name(), test.ShouldResemble, name)
	}
}

func BenchmarkUseMLVisionModel(b *testing.B) {
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(b, err, test.ShouldBeNil)
	test.That(b, pic, test.ShouldNotBeNil)
	name := mlmodel.Named("myMLModel")
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	ctx := context.Background()
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, name)
	test.That(b, err, test.ShouldBeNil)
	test.That(b, out, test.ShouldNotBeNil)
	modelCfg := MLModelConfig{ModelName: name.Name}

	service, err := registerMLModelVisionService(ctx, name, &modelCfg, &inject.Robot{}, logging.NewLogger("benchmark"))
	test.That(b, err, test.ShouldBeNil)
	test.That(b, service, test.ShouldNotBeNil)
	test.That(b, service.Name(), test.ShouldResemble, name)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Detections should be worst case (more to unpack)
		detections, err := service.Detections(ctx, pic, nil)
		test.That(b, err, test.ShouldBeNil)
		test.That(b, detections, test.ShouldNotBeNil)
	}
}

func getTestMlModel(modelLoc string) (mlmodel.Service, error) {
	ctx := context.Background()
	testMLModelServiceName := "test-model"

	name := mlmodel.Named(testMLModelServiceName)
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	return tflitecpu.NewTFLiteCPUModel(ctx, &cfg, name)
}

func TestAddingIncorrectModelTypeToModel(t *testing.T) {
	modelLocDetector := artifact.MustPath("vision/tflite/effdet0.tflite")
	ctx := context.Background()

	// get detector model
	mlm, err := getTestMlModel(modelLocDetector)
	test.That(t, err, test.ShouldBeNil)

	nameMap := &sync.Map{}
	classifier, err := attemptToBuildClassifier(mlm, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, classifier, test.ShouldNotBeNil)

	err = checkIfClassifierWorks(ctx, classifier)
	test.That(t, err, test.ShouldNotBeNil)

	detector, err := attemptToBuildDetector(mlm, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, detector, test.ShouldNotBeNil)

	err = checkIfDetectorWorks(ctx, detector)
	test.That(t, err, test.ShouldBeNil)

	modelLocClassifier := artifact.MustPath("vision/tflite/mobilenetv2_class.tflite")

	mlm, err = getTestMlModel(modelLocClassifier)
	test.That(t, err, test.ShouldBeNil)

	nameMap = &sync.Map{}
	classifier, err = attemptToBuildClassifier(mlm, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, classifier, test.ShouldNotBeNil)

	err = checkIfClassifierWorks(ctx, classifier)
	test.That(t, err, test.ShouldBeNil)

	mlm, err = getTestMlModel(modelLocClassifier)
	test.That(t, err, test.ShouldBeNil)

	detector, err = attemptToBuildDetector(mlm, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, detector, test.ShouldNotBeNil)

	err = checkIfDetectorWorks(ctx, detector)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestNewMLDetector(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	// Set it up as a ML Model

	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	labelLoc := artifact.MustPath("vision/tflite/effdetlabels.txt")
	cfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  labelLoc,
	}
	noLabelCfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
	}

	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	// Test that a detector would give the expected output on the dog image
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("myMLDet"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	check, err := out.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, check.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, check.Outputs[0].Name, test.ShouldResemble, "location")
	test.That(t, check.Outputs[1].Name, test.ShouldResemble, "category")
	test.That(t, check.Outputs[0].Extra["labels"], test.ShouldNotBeNil)

	nameMap := &sync.Map{}
	gotDetector, err := attemptToBuildDetector(out, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err := gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.789)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.7)
	test.That(t, gotDetections[0].BoundingBox().Min.X, test.ShouldBeGreaterThan, 124)
	test.That(t, gotDetections[0].BoundingBox().Min.X, test.ShouldBeLessThan, 127)
	test.That(t, gotDetections[0].BoundingBox().Min.Y, test.ShouldBeGreaterThan, 40)
	test.That(t, gotDetections[0].BoundingBox().Min.Y, test.ShouldBeLessThan, 44)
	test.That(t, gotDetections[0].BoundingBox().Max.X, test.ShouldBeGreaterThan, 196)
	test.That(t, gotDetections[0].BoundingBox().Max.X, test.ShouldBeLessThan, 202)
	test.That(t, gotDetections[0].BoundingBox().Max.Y, test.ShouldBeGreaterThan, 158)
	test.That(t, gotDetections[0].BoundingBox().Max.Y, test.ShouldBeLessThan, 163)

	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "Dog")

	// Ensure that the same model without labelpath responds similarly
	outNL, err := tflitecpu.NewTFLiteCPUModel(ctx, &noLabelCfg, mlmodel.Named("myOtherMLDet"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outNL, test.ShouldNotBeNil)
	nameMap = &sync.Map{}
	gotDetectorNL, err := attemptToBuildDetector(outNL, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectorNL, test.ShouldNotBeNil)
	gotDetectionsNL, err := gotDetectorNL(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectionsNL[0].Score(), test.ShouldBeGreaterThan, 0.789)
	test.That(t, gotDetectionsNL[1].Score(), test.ShouldBeGreaterThan, 0.7)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.X, test.ShouldBeGreaterThan, 124)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.X, test.ShouldBeLessThan, 127)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.Y, test.ShouldBeGreaterThan, 40)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.Y, test.ShouldBeLessThan, 44)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.X, test.ShouldBeGreaterThan, 196)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.X, test.ShouldBeLessThan, 202)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.Y, test.ShouldBeGreaterThan, 158)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.Y, test.ShouldBeLessThan, 163)

	test.That(t, gotDetectionsNL[0].Label(), test.ShouldResemble, "17")
	test.That(t, gotDetectionsNL[1].Label(), test.ShouldResemble, "17")
}

func TestNewMLClassifier(t *testing.T) {
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effnet0.tflite")
	labelLoc := artifact.MustPath("vision/tflite/imagenetlabels.txt")

	cfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  labelLoc,
	}
	noLabelCfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	// Test that a classifier would give the expected result on the lion image
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("myMLClassif"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	check, err := out.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, check.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, check.Outputs[0].Name, test.ShouldResemble, "probability")
	test.That(t, check.Outputs[0].Extra["labels"], test.ShouldNotBeNil)

	nameMap := &sync.Map{}
	gotClassifier, err := attemptToBuildClassifier(out, nameMap)
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
	outNL, err := tflitecpu.NewTFLiteCPUModel(ctx, &noLabelCfg, mlmodel.Named("myOtherMLClassif"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outNL, test.ShouldNotBeNil)
	nameMap = &sync.Map{}
	gotClassifierNL, err := attemptToBuildClassifier(outNL, nameMap)
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

func TestMoreMLDetectors(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	name := mlmodel.Named("ssd")
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/ssdmobilenet.tflite")
	labelLoc := artifact.MustPath("vision/tflite/effdetlabels.txt")
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  labelLoc,
	}

	// Test that a detector would give the expected output on the dog image
	outModel, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outModel, test.ShouldNotBeNil)
	check, err := outModel.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	// Even without metadata we should find
	test.That(t, check.Inputs[0].Shape, test.ShouldResemble, []int{1, 320, 320, 3})
	test.That(t, check.Inputs[0].DataType, test.ShouldResemble, "float32")
	test.That(t, len(check.Outputs), test.ShouldEqual, 4)

	nameMap := &sync.Map{}
	gotDetector, err := attemptToBuildDetector(outModel, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err := gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.82)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.8)
	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "Dog")
}

func TestMoreMLClassifiers(t *testing.T) {
	// Test that mobileNet classifier gives expected output on the redpanda image
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/mobilenetv2_class.tflite")
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/redpanda.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	outModel, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("mobileNet"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outModel, test.ShouldNotBeNil)
	check, err := outModel.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	nameMap := &sync.Map{}
	gotClassifier, err := attemptToBuildClassifier(outModel, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifier, test.ShouldNotBeNil)

	gotClassifications, err := gotClassifier(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	bestClass, err := gotClassifications.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bestClass[0].Label(), test.ShouldResemble, "390")
	test.That(t, bestClass[0].Score(), test.ShouldBeGreaterThan, 0.93)

	// Test that mobileNet imageNet classifier gives expected output on lion image
	modelLoc = artifact.MustPath("vision/tflite/mobilenetv2_imagenet.tflite")
	pic, err = rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)
	cfg = tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	outModel, err = tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("mobileNet"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outModel, test.ShouldNotBeNil)
	check, err = outModel.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)

	nameMap = &sync.Map{}
	gotClassifier, err = attemptToBuildClassifier(outModel, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifier, test.ShouldNotBeNil)
	gotClassifications, err = gotClassifier(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifications, test.ShouldNotBeNil)
	bestClass, err = gotClassifications.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bestClass[0].Label(), test.ShouldResemble, "292")
	test.That(t, bestClass[0].Score(), test.ShouldBeGreaterThan, 0.93)
}

func TestLabelReader(t *testing.T) {
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	labelLoc := artifact.MustPath("vision/tflite/fakelabels.txt")
	cfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  labelLoc,
	}
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("fakeLabels"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	outMD, err := out.Metadata(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outMD, test.ShouldNotBeNil)
	outLabels := getLabelsFromMetadata(outMD)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outLabels[0], test.ShouldResemble, "this")
	test.That(t, outLabels[1], test.ShouldResemble, "could")
	test.That(t, outLabels[2], test.ShouldResemble, "be")
	test.That(t, len(outLabels), test.ShouldEqual, 12)
}

func TestSpaceDelineatedLabels(t *testing.T) {
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	labelLoc := artifact.MustPath("vision/classification/lorem.txt")
	cfg := tflitecpu.TFLiteConfig{ // detector config
		ModelPath:  modelLoc,
		NumThreads: 2,
		LabelPath:  labelLoc,
	}
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("spacedLabels"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	outMD, err := out.Metadata(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outMD, test.ShouldNotBeNil)
	outLabels := getLabelsFromMetadata(outMD)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(outLabels), test.ShouldEqual, 10)
}

func TestOneClassifierOnManyCameras(t *testing.T) {
	ctx := context.Background()

	// Test that one classifier can be used in two goroutines
	picPanda, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/redpanda.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	picLion, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/lion.jpeg"))
	test.That(t, err, test.ShouldBeNil)

	modelLoc := artifact.MustPath("vision/tflite/mobilenetv2_class.tflite")
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}

	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("testClassifier"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	nameMap := &sync.Map{}
	outClassifier, err := attemptToBuildClassifier(out, nameMap)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outClassifier, test.ShouldNotBeNil)
	valuePanda, valueLion := classifyTwoImages(picPanda, picLion, outClassifier)
	test.That(t, valuePanda, test.ShouldNotBeNil)
	test.That(t, valueLion, test.ShouldNotBeNil)
}

func TestMultipleClassifiersOneModel(t *testing.T) {
	modelLoc := artifact.MustPath("vision/tflite/mobilenetv2_class.tflite")
	cfg := tflitecpu.TFLiteConfig{
		ModelPath:  modelLoc,
		NumThreads: 2,
	}
	ctx := context.Background()
	out, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("testClassifier"))
	test.That(t, err, test.ShouldBeNil)

	nameMap := &sync.Map{}
	Classifier1, err := attemptToBuildClassifier(out, nameMap)
	test.That(t, err, test.ShouldBeNil)

	nameMap = &sync.Map{}
	Classifier2, err := attemptToBuildClassifier(out, nameMap)
	test.That(t, err, test.ShouldBeNil)

	picPanda, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/redpanda.jpeg"))
	test.That(t, err, test.ShouldBeNil)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		getNClassifications(ctx, t, picPanda, 10, Classifier1)
	}()

	go func() {
		defer wg.Done()
		getNClassifications(ctx, t, picPanda, 10, Classifier2)
	}()

	wg.Wait()
}

func classifyTwoImages(picPanda, picLion *rimage.Image,
	got classification.Classifier,
) (classification.Classifications, classification.Classifications) {
	resultPanda := make(chan classification.Classifications)
	resultLion := make(chan classification.Classifications)

	go gotWithCallback(picPanda, resultPanda, got)
	go gotWithCallback(picLion, resultLion, got)

	valuePanda := <-resultPanda
	valueLion := <-resultLion

	close(resultPanda)
	close(resultLion)

	return valuePanda, valueLion
}

func gotWithCallback(img *rimage.Image, result chan classification.Classifications, got classification.Classifier) {
	classifications, _ := got(context.Background(), img)
	result <- classifications
}

func getNClassifications(
	ctx context.Context,
	t *testing.T,
	img *rimage.Image,
	n int,
	c classification.Classifier,
) {
	t.Helper()
	results := make([]classification.Classifications, n)
	var err error

	for i := 0; i < n; i++ {
		results[i], err = c(ctx, img)
		test.That(t, err, test.ShouldBeNil)
		res, err := results[i].TopN(1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res[0].Score(), test.ShouldNotBeNil)
	}
}

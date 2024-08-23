package mlvision

import (
	"context"
	"sync"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"gorgonia.org/tensor"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/mlmodel/tflitecpu"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/classification"
)

func mockEffDetModel(name string, labelLoc string) mlmodel.Service {
	// using the effdet0.tflite model as a template
	// pretend it has taken in the picture of "vision/tflite/dogscute.jpeg"
	effDetMock := inject.NewMLModelService(name)
	md := mlmodel.MLMetadata{
		ModelName:        "EfficientDet Lite0 V1",
		ModelType:        "tflite_detector",
		ModelDescription: "Identify which of a known set of objects might be present and provide information about their positions within the given image or a video stream.",
	}
	inputs := make([]mlmodel.TensorInfo, 0, 1)
	imageIn := mlmodel.TensorInfo{
		Name:        "image",
		Description: "Input image to be detected. The expected image is 320 x 320, with three channels (red, blue, and green) per pixel. Each value in the tensor is a single byte between 0 and 255.",
		DataType:    "uint8",
		Shape:       []int{1, 320, 320, 3},
	}
	inputs = append(inputs, imageIn)
	md.Inputs = inputs
	outputs := make([]mlmodel.TensorInfo, 0, 4)
	locationOut := mlmodel.TensorInfo{
		Name:        "location",
		Description: "The locations of the detected boxes.",
		DataType:    "float32",
	}
	if labelLoc != "" {
		extra := map[string]interface{}{"labels": labelLoc}
		locationOut.Extra = extra
	}
	outputs = append(outputs, locationOut)
	categoryOut := mlmodel.TensorInfo{
		Name:        "category",
		Description: "The categories of the detected boxes.",
		DataType:    "float32",
	}
	outputs = append(outputs, categoryOut)
	scoreOut := mlmodel.TensorInfo{
		Name:        "score",
		Description: "The scores of the detected boxes.",
		DataType:    "float32",
	}
	outputs = append(outputs, scoreOut)
	numberOut := mlmodel.TensorInfo{
		Name:        "number of detections",
		Description: "The number of the detected boxes.",
		DataType:    "float32",
	}
	outputs = append(outputs, numberOut)
	md.Outputs = outputs
	effDetMock.MetadataFunc = func(ctx context.Context) (mlmodel.MLMetadata, error) {
		return md, nil
	}

	// now define the output tensors
	outputInfer := ml.Tensors{}
	//score
	score := []float32{0.81640625, 0.6875, 0.109375, 0.09375, 0.0625,
		0.0546875, 0.05078125, 0.0390625, 0.03515625, 0.03125,
		0.0234375, 0.0234375, 0.0234375, 0.0234375, 0.01953125,
		0.01953125, 0.01953125, 0.01953125, 0.01953125, 0.01953125,
		0.015625, 0.015625, 0.015625, 0.015625, 0.015625}
	scoreTensor := tensor.New(tensor.WithShape(1, 25), tensor.WithBacking(score))
	outputInfer["score"] = scoreTensor
	// nDetections
	nDetections := []float32{25}
	detectionTensor := tensor.New(tensor.WithShape(1), tensor.WithBacking(nDetections))
	outputInfer["number of detections"] = detectionTensor
	// locations
	locations := []float32{
		0.20903039, 0.49185863, 0.82770026, 0.7690754,
		0.2376312, 0.260224, 0.82330287, 0.5374408,
		0.21014652, 0.37334082, 0.82086015, 0.67316914,
		0.9004202, 0.36880112, 0.95539546, 0.41990197,
		0.19502541, 0.1988186, 0.8602221, 0.77355766,
		0.836329, 0.86517155, 0.8984374, 0.99401116,
		0.2503236, 0.2755023, 0.56928396, 0.50930154,
		0.4401425, 0.35509717, 0.53873336, 0.41215116,
		0.22128013, 0.51680136, 0.5461217, 0.7506006,
		0.89365757, 0.6519017, 0.9923049, 0.7121358,
		0.34879953, 0.47103795, 0.45682132, 0.50783795,
		0.83736897, 0.94356436, 0.89037156, 0.98691684,
		0.25913447, 0.12777925, 0.7270005, 0.6214407,
		0.44479424, 0.21759495, 0.81613976, 0.6628721,
		0.38580972, 0.5132986, 0.5085694, 0.5617015,
		0.49028072, 0.00190118, 0.59634674, 0.02697465,
		0.5979702, 0.9293068, 0.7516399, 0.99569315,
		0.8964205, 0.33521998, 0.95665455, 0.4144457,
		0.4158226, 0.2888925, 0.5341774, 0.46885914,
		0.20846531, 0.2381043, 0.50130117, 0.6228298,
		0.38078213, 0.34770778, 0.5372853, 0.4334447,
		0.4441566, 0.45994544, 0.5502226, 0.50924087,
		0.5679829, 0.98425895, 0.76903045, 0.9965547,
		0.6335254, 0.97844476, 0.76085377, 0.9946173,
		0.8215679, 0.07016394, 0.89795077, 0.11853918}
	locationTensor := tensor.New(tensor.WithShape(1, 25, 4), tensor.WithBacking(locations))
	outputInfer["location"] = locationTensor
	// categories
	categories := []float32{17., 17., 17., 36., 17., 87., 17., 33., 17., 36., 33., 87., 17.,
		17., 33., 0., 0., 36., 33., 17., 17., 33., 0., 0., 36.}
	categoryTensor := tensor.New(tensor.WithShape(1, 25), tensor.WithBacking(categories))
	outputInfer["category"] = categoryTensor
	effDetMock.InferFunc = func(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error) {
		return outputInfer, nil
	}
	effDetMock.CloseFunc = func(ctx context.Context) error {
		return nil
	}
	return effDetMock
}
func BenchmarkAddMLVisionModel(b *testing.B) {
	ctx := context.Background()
	out := mockEffDetModel("myMLModel", "")
	name := out.Name()
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
	ctx := context.Background()
	out := mockEffDetModel("myMLModel", "")
	name := out.Name()
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(b, err, test.ShouldBeNil)
	test.That(b, pic, test.ShouldNotBeNil)
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
	// get detector model
	ctx := context.Background()
	mlm := mockEffDetModel("test-model", "")

	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	conf := &MLModelConfig{}
	classifier, err := attemptToBuildClassifier(mlm, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, classifier, test.ShouldNotBeNil)

	err = checkIfClassifierWorks(ctx, classifier)
	test.That(t, err, test.ShouldNotBeNil)

	detector, err := attemptToBuildDetector(mlm, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, detector, test.ShouldNotBeNil)

	err = checkIfDetectorWorks(ctx, detector)
	test.That(t, err, test.ShouldBeNil)

	modelLocClassifier := artifact.MustPath("vision/tflite/mobilenetv2_class.tflite")

	mlm, err = getTestMlModel(modelLocClassifier)
	test.That(t, err, test.ShouldBeNil)

	inNameMap = &sync.Map{}
	outNameMap = &sync.Map{}
	classifier, err = attemptToBuildClassifier(mlm, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, classifier, test.ShouldNotBeNil)

	err = checkIfClassifierWorks(ctx, classifier)
	test.That(t, err, test.ShouldBeNil)

	mlm, err = getTestMlModel(modelLocClassifier)
	test.That(t, err, test.ShouldBeNil)

	detector, err = attemptToBuildDetector(mlm, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, detector, test.ShouldNotBeNil)

	err = checkIfDetectorWorks(ctx, detector)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestNewMLDetector(t *testing.T) {
	// Test that a detector would give an expected output on the dog image
	// Set it up as a ML Model
	ctx := context.Background()
	labelLoc := artifact.MustPath("vision/tflite/effdetlabels.txt")
	out := mockEffDetModel("test-model", labelLoc)

	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	// Test that a detector would give the expected output on the dog image
	check, err := out.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, check.Inputs[0].Name, test.ShouldResemble, "image")
	test.That(t, check.Outputs[0].Name, test.ShouldResemble, "location")
	test.That(t, check.Outputs[1].Name, test.ShouldResemble, "category")
	test.That(t, check.Outputs[0].Extra["labels"], test.ShouldNotBeNil)

	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	conf := &MLModelConfig{}
	gotDetector, err := attemptToBuildDetector(out, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err := gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetections[0].Score(), test.ShouldAlmostEqual, 0.81640625)
	test.That(t, gotDetections[1].Score(), test.ShouldAlmostEqual, 0.6875)
	test.That(t, gotDetections[0].BoundingBox().Min.X, test.ShouldBeGreaterThan, 124)
	test.That(t, gotDetections[0].BoundingBox().Min.X, test.ShouldBeLessThan, 127)
	test.That(t, gotDetections[0].BoundingBox().Min.Y, test.ShouldEqual, 40)
	test.That(t, gotDetections[0].BoundingBox().Max.X, test.ShouldBeGreaterThan, 196)
	test.That(t, gotDetections[0].BoundingBox().Max.X, test.ShouldBeLessThan, 202)
	test.That(t, gotDetections[0].BoundingBox().Max.Y, test.ShouldBeGreaterThan, 158)
	test.That(t, gotDetections[0].BoundingBox().Max.Y, test.ShouldBeLessThan, 163)

	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "Dog")

	// Ensure that the same model without labelpath responds similarly
	outNL := mockEffDetModel("test-model", "")
	inNameMap = &sync.Map{}
	outNameMap = &sync.Map{}
	conf = &MLModelConfig{}
	gotDetectorNL, err := attemptToBuildDetector(outNL, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectorNL, test.ShouldNotBeNil)
	gotDetectionsNL, err := gotDetectorNL(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectionsNL[0].Score(), test.ShouldAlmostEqual, 0.81640625)
	test.That(t, gotDetectionsNL[1].Score(), test.ShouldAlmostEqual, 0.6875)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.X, test.ShouldBeGreaterThan, 124)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.X, test.ShouldBeLessThan, 127)
	test.That(t, gotDetectionsNL[0].BoundingBox().Min.Y, test.ShouldEqual, 40)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.X, test.ShouldBeGreaterThan, 196)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.X, test.ShouldBeLessThan, 202)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.Y, test.ShouldBeGreaterThan, 158)
	test.That(t, gotDetectionsNL[0].BoundingBox().Max.Y, test.ShouldBeLessThan, 163)

	test.That(t, gotDetectionsNL[0].Label(), test.ShouldResemble, "17")
	test.That(t, gotDetectionsNL[1].Label(), test.ShouldResemble, "17")

	// Ensure that the same model without labels but with vision service labels works
	conf = &MLModelConfig{
		LabelPath: labelLoc,
	}
	gotDetectorNL, err = attemptToBuildDetector(outNL, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectorNL, test.ShouldNotBeNil)
	gotDetectionsNL, err = gotDetectorNL(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetectionsNL[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetectionsNL[1].Label(), test.ShouldResemble, "Dog")
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

	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	conf := &MLModelConfig{}
	gotClassifier, err := attemptToBuildClassifier(out, inNameMap, outNameMap, conf)
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
	inNameMap = &sync.Map{}
	outNameMap = &sync.Map{}
	conf = &MLModelConfig{}
	gotClassifierNL, err := attemptToBuildClassifier(outNL, inNameMap, outNameMap, conf)
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

	// Ensure that vision service label_path loads
	conf = &MLModelConfig{
		LabelPath: labelLoc,
	}
	gotClassifierNL, err = attemptToBuildClassifier(outNL, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifierNL, test.ShouldNotBeNil)
	gotClassificationsNL, err = gotClassifierNL(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassificationsNL, test.ShouldNotBeNil)
	topNL, err = gotClassificationsNL.TopN(5)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, topNL, test.ShouldNotBeNil)
	test.That(t, topNL[0].Label(), test.ShouldContainSubstring, "lion")
}

func TestMLDetectorWithNoCategory(t *testing.T) {
	// Test that a detector would give an expected output on the person
	// This detector only has two output tensors, Identity (location) and Identity_1 (score)
	pic, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/person.jpg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pic, test.ShouldNotBeNil)

	name := mlmodel.Named("yolo_person")
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/yolov4-tiny-416_person.tflite")
	cfg := tflitecpu.TFLiteConfig{
		ModelPath: modelLoc,
	}

	// Test that a detector would give the expected output on the dog image
	outModel, err := tflitecpu.NewTFLiteCPUModel(ctx, &cfg, name)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, outModel, test.ShouldNotBeNil)
	check, err := outModel.Metadata(ctx)
	test.That(t, check, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeNil)
	// Even without metadata we should find
	test.That(t, check.Inputs[0].Shape, test.ShouldResemble, []int{1, 416, 416, 3})
	test.That(t, check.Inputs[0].DataType, test.ShouldResemble, "float32")
	test.That(t, len(check.Outputs), test.ShouldEqual, 2) // only two output tensors

	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	outNameMap.Store("location", "Identity")
	outNameMap.Store("score", "Identity_1")
	conf := &MLModelConfig{}
	gotDetector, err := attemptToBuildDetector(outModel, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err := gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetections[2297].Score(), test.ShouldBeGreaterThan, 0.7)
	test.That(t, gotDetections[2297].Label(), test.ShouldResemble, "0")
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

	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	conf := &MLModelConfig{}
	gotDetector, err := attemptToBuildDetector(outModel, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err := gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gotDetections), test.ShouldEqual, 10)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, 0.82)
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, 0.8)
	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "Dog")

	// test filters
	// add min confidence first
	minConf := 0.81
	conf = &MLModelConfig{DefaultConfidence: minConf}
	gotDetector, err = attemptToBuildDetector(outModel, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err = gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gotDetections), test.ShouldEqual, 3)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, minConf)

	// then add label filter
	labelMap := map[string]float64{"DOG": 0.8, "CARROT": 0.3}
	conf = &MLModelConfig{DefaultConfidence: minConf, LabelConfidenceMap: labelMap}
	gotDetector, err = attemptToBuildDetector(outModel, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotDetector, test.ShouldNotBeNil)

	gotDetections, err = gotDetector(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gotDetections), test.ShouldEqual, 3)
	test.That(t, gotDetections[0].Score(), test.ShouldBeGreaterThan, labelMap["DOG"])
	test.That(t, gotDetections[0].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[1].Score(), test.ShouldBeGreaterThan, labelMap["DOG"])
	test.That(t, gotDetections[1].Label(), test.ShouldResemble, "Dog")
	test.That(t, gotDetections[2].Score(), test.ShouldBeGreaterThan, labelMap["CARROT"])
	test.That(t, gotDetections[2].Label(), test.ShouldResemble, "Carrot")
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

	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	conf := &MLModelConfig{}
	gotClassifier, err := attemptToBuildClassifier(outModel, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifier, test.ShouldNotBeNil)

	gotClassifications, err := gotClassifier(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	bestClass, err := gotClassifications.TopN(1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bestClass[0].Label(), test.ShouldResemble, "390")
	test.That(t, bestClass[0].Score(), test.ShouldBeGreaterThan, 0.93)
	// add min confidence first
	minConf := 0.05
	conf = &MLModelConfig{DefaultConfidence: minConf}
	gotClassifier, err = attemptToBuildClassifier(outModel, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifier, test.ShouldNotBeNil)

	gotClassifications, err = gotClassifier(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gotClassifications), test.ShouldEqual, 1)
	test.That(t, gotClassifications[0].Score(), test.ShouldBeGreaterThan, minConf)

	// then add label filter
	labelMap := map[string]float64{"390": 0.8}
	conf = &MLModelConfig{DefaultConfidence: minConf, LabelConfidenceMap: labelMap}
	gotClassifier, err = attemptToBuildClassifier(outModel, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotClassifier, test.ShouldNotBeNil)

	gotClassifications, err = gotClassifier(ctx, pic)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(gotClassifications), test.ShouldEqual, 1)
	test.That(t, gotClassifications[0].Score(), test.ShouldBeGreaterThan, labelMap["390"])
	test.That(t, gotClassifications[0].Label(), test.ShouldResemble, "390")

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

	inNameMap = &sync.Map{}
	outNameMap = &sync.Map{}
	conf = &MLModelConfig{}
	gotClassifier, err = attemptToBuildClassifier(outModel, inNameMap, outNameMap, conf)
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
	outLabels := getLabelsFromMetadata(outMD, "")
	test.That(t, len(outLabels), test.ShouldEqual, 12)
	test.That(t, outLabels[0], test.ShouldResemble, "this")
	test.That(t, outLabels[1], test.ShouldResemble, "could")
	test.That(t, outLabels[2], test.ShouldResemble, "be")
	altLabelLoc := artifact.MustPath("vision/tflite/fakelabels_alt.txt")
	outLabels = getLabelsFromMetadata(outMD, altLabelLoc)
	test.That(t, len(outLabels), test.ShouldEqual, 12)
	test.That(t, outLabels[0], test.ShouldResemble, "alt_this")
	test.That(t, outLabels[1], test.ShouldResemble, "alt_could")
	test.That(t, outLabels[2], test.ShouldResemble, "alt_be")
}

func TestBlankLabelLines(t *testing.T) {
	ctx := context.Background()
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	labelLoc := artifact.MustPath("vision/tflite/effdetlabels_with_spaces.txt")
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
	outLabels := getLabelsFromMetadata(outMD, "")
	test.That(t, len(outLabels), test.ShouldEqual, 91)
	test.That(t, outLabels[0], test.ShouldResemble, "Person")
	test.That(t, outLabels[1], test.ShouldResemble, "Bicycle")
	test.That(t, outLabels[2], test.ShouldResemble, "Car")

	labelLoc2 := artifact.MustPath("vision/tflite/empty_labels.txt")
	cfg.LabelPath = labelLoc2
	out, err = tflitecpu.NewTFLiteCPUModel(ctx, &cfg, mlmodel.Named("emptyLabels"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out, test.ShouldNotBeNil)
	outMD, err = out.Metadata(ctx)
	test.That(t, err, test.ShouldBeNil)
	outLabels = getLabelsFromMetadata(outMD, "")
	test.That(t, outLabels, test.ShouldBeNil)
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
	outLabels := getLabelsFromMetadata(outMD, "")
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
	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	conf := &MLModelConfig{}
	outClassifier, err := attemptToBuildClassifier(out, inNameMap, outNameMap, conf)
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

	inNameMap := &sync.Map{}
	outNameMap := &sync.Map{}
	conf := &MLModelConfig{}
	Classifier1, err := attemptToBuildClassifier(out, inNameMap, outNameMap, conf)
	test.That(t, err, test.ShouldBeNil)

	inNameMap = &sync.Map{}
	outNameMap = &sync.Map{}
	conf = &MLModelConfig{}
	Classifier2, err := attemptToBuildClassifier(out, inNameMap, outNameMap, conf)
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

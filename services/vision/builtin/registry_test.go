package builtin

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/services/vision"
	vis "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func TestDetectorMap(t *testing.T) {
	fn := func(context.Context, image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	registeredFn := registeredModel{Model: fn, Closer: nil, ModelType: ColorDetector}
	emptyFn := registeredModel{Model: nil, Closer: nil}
	fnName := "x"
	reg := make(modelMap)
	testlog := golog.NewLogger("testlog")
	// no detector
	err := reg.RegisterVisModel(fnName, &emptyFn, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot register a nil model")
	// success
	reg.RegisterVisModel(fnName, &registeredFn, testlog)
	// detector names
	names := reg.DetectorNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	det, err := reg.modelLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, det.Model, test.ShouldEqual, fn)
	det, err = reg.modelLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such vision model with name")
	test.That(t, det.Model, test.ShouldBeNil)
	// duplicate
	err = reg.RegisterVisModel(fnName, &registeredFn, testlog)
	test.That(t, err, test.ShouldBeNil)
	names = reg.DetectorNames()
	test.That(t, names, test.ShouldContain, fnName)
	// remove
	err = reg.removeVisModel(fnName, testlog)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reg.DetectorNames(), test.ShouldNotContain, fnName)
}

func TestCloser(t *testing.T) {
	fakeDetectFn := func(context.Context, image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	closer := inf.TFLiteStruct{Info: &inf.TFLiteInfo{100, 100, 3, []int{1, 100, 100, 3}, "uint8", 1, 4, []string{}}}

	d := registeredModel{Model: fakeDetectFn, Closer: &closer, ModelType: ColorDetector}
	reg := make(modelMap)
	err := reg.RegisterVisModel("x", &d, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	got := reg["x"].Closer
	test.That(t, got, test.ShouldNotBeNil)

	fakeClassifyFn := func(context.Context, image.Image) (classification.Classifications, error) {
		return []classification.Classification{classification.NewClassification(0.0, "nothing")}, nil
	}
	d = registeredModel{Model: fakeClassifyFn, Closer: &closer, ModelType: TFLiteClassifier}
	err = reg.RegisterVisModel("y", &d, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	got = reg["y"].Closer
	test.That(t, got, test.ShouldNotBeNil)
}

func TestDetectorRemoval(t *testing.T) {
	fakeDetectFn := func(context.Context, image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	ctx := context.Background()
	closer, err := addTFLiteModel(ctx, artifact.MustPath("vision/tflite/effdet0.tflite"), nil)
	test.That(t, err, test.ShouldBeNil)
	d := registeredModel{Model: fakeDetectFn, Closer: closer, ModelType: TFLiteDetector}
	testlog := golog.NewTestLogger(t)
	reg := make(modelMap)
	err = reg.RegisterVisModel("x", &d, testlog)
	test.That(t, err, test.ShouldBeNil)
	err = reg.RegisterVisModel("y", &d, testlog)
	test.That(t, err, test.ShouldBeNil)
	logger, obs := golog.NewObservedTestLogger(t)
	err = reg.removeVisModel("z", logger)
	test.That(t, err, test.ShouldBeNil)
	got := obs.All()[len(obs.All())-1].Message
	test.That(t, got, test.ShouldContainSubstring, "no such vision model with name")
	err = reg.removeVisModel("x", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reg.DetectorNames(), test.ShouldNotContain, "x")
}

func TestRegisterTFLiteDetector(t *testing.T) {
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	conf := &vision.Attributes{
		ModelRegistry: []vision.VisModelConfig{
			{
				Name: "my_tflite_det",
				Type: "tflite_detector",
				Parameters: config.AttributeMap{
					"model_path":  modelLoc,
					"label_path":  "",
					"num_threads": 1,
				},
			},
		},
	}
	reg := make(modelMap)
	err := registerNewVisModels(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
}

func TestRegisterTensorFlowDetector(t *testing.T) {
	conf := &vision.Attributes{
		ModelRegistry: []vision.VisModelConfig{
			{
				Name:       "my_tensorflow_det",
				Type:       "tf_detector",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(modelMap)
	err := registerNewVisModels(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, newVisModelTypeNotImplemented("tf_detector"))
}

func TestRegisterColorDetector(t *testing.T) {
	conf := &vision.Attributes{
		ModelRegistry: []vision.VisModelConfig{
			{
				Name: "my_color_det",
				Type: "color_detector",
				Parameters: config.AttributeMap{
					"segment_size_px": 150000,
					"tolerance_pct":   0.44,
					"detect_color":    "#4F3815",
				},
			},
		},
	}
	reg := make(modelMap)
	err := registerNewVisModels(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	_, err = reg.modelLookup("my_color_det")
	test.That(t, err, test.ShouldBeNil)

	// error from bad config
	conf.ModelRegistry[0].Parameters = nil
	err = registerNewVisModels(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")
}

func TestRegisterUnknown(t *testing.T) {
	conf := &vision.Attributes{
		ModelRegistry: []vision.VisModelConfig{
			{
				Name:       "my_random_det",
				Type:       "not_real",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(modelMap)
	err := registerNewVisModels(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, newVisModelTypeNotImplemented("not_real"))
}

func TestClassifierMap(t *testing.T) {
	fn := func(context.Context, image.Image) (classification.Classifications, error) {
		return []classification.Classification{classification.NewClassification(0.0, "nothing")}, nil
	}
	registeredFn := registeredModel{Model: fn, Closer: nil, ModelType: TFLiteClassifier}
	emptyFn := registeredModel{Model: nil, Closer: nil}
	fnName := "x"
	reg := make(modelMap)
	testlog := golog.NewLogger("testlog")
	// no classifier (empty model)
	err := reg.RegisterVisModel(fnName, &emptyFn, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot register a nil model")
	// success
	reg.RegisterVisModel(fnName, &registeredFn, testlog)
	// classifier names
	names := reg.ClassifierNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	c, err := reg.modelLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, c.Model, test.ShouldEqual, fn)
	c, err = reg.modelLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such vision model with name")
	test.That(t, c.Model, test.ShouldBeNil)
	// duplicate
	err = reg.RegisterVisModel(fnName, &registeredFn, testlog)
	test.That(t, err, test.ShouldBeNil)
	names = reg.ClassifierNames()
	test.That(t, names, test.ShouldContain, fnName)
	// remove
	err = reg.removeVisModel(fnName, testlog)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reg.ClassifierNames(), test.ShouldNotContain, fnName)
}

func TestClassifierRemoval(t *testing.T) {
	fakeClassifyFn := func(context.Context, image.Image) (classification.Classifications, error) {
		return []classification.Classification{classification.NewClassification(0.0, "nothing")}, nil
	}
	ctx := context.Background()
	closer, err := addTFLiteModel(ctx, artifact.MustPath("vision/tflite/effnet0.tflite"), nil)
	test.That(t, err, test.ShouldBeNil)
	d := registeredModel{Model: fakeClassifyFn, Closer: closer, ModelType: TFLiteClassifier}
	testlog := golog.NewTestLogger(t)
	reg := make(modelMap)
	err = reg.RegisterVisModel("x", &d, testlog)
	test.That(t, err, test.ShouldBeNil)
	err = reg.RegisterVisModel("y", &d, testlog)
	test.That(t, err, test.ShouldBeNil)
	logger, obs := golog.NewObservedTestLogger(t)
	err = reg.removeVisModel("z", logger)
	test.That(t, err, test.ShouldBeNil)
	got := obs.All()[len(obs.All())-1].Message
	test.That(t, got, test.ShouldContainSubstring, "no such vision model with name")
	err = reg.removeVisModel("x", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reg.ClassifierNames(), test.ShouldNotContain, "x")
}

func TestRegisterTFLiteClassifier(t *testing.T) {
	modelLoc := artifact.MustPath("vision/tflite/effnet0.tflite")
	conf := &vision.Attributes{
		ModelRegistry: []vision.VisModelConfig{
			{
				Name: "my_tflite_classif",
				Type: "tflite_classifier",
				Parameters: config.AttributeMap{
					"model_path":  modelLoc,
					"label_path":  "",
					"num_threads": 1,
				},
			},
		},
	}
	reg := make(modelMap)
	err := registerNewVisModels(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
}

func TestRegisterTensorFlowClassifier(t *testing.T) {
	conf := &vision.Attributes{
		ModelRegistry: []vision.VisModelConfig{
			{
				Name:       "tensorflow_classif",
				Type:       "tf_classifier",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(modelMap)
	err := registerNewVisModels(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, newVisModelTypeNotImplemented("tf_classifier"))
}

func TestSegmenterMap(t *testing.T) {
	fn := func(ctx context.Context, c camera.Camera) ([]*vis.Object, error) {
		return []*vis.Object{vis.NewEmptyObject()}, nil
	}
	fnName := "x"
	segMap := make(modelMap)
	testlog := golog.NewLogger("testlog")
	// no segmenter
	noSeg := registeredModel{Model: nil, ModelType: RCSegmenter}
	err := segMap.RegisterVisModel(fnName, &noSeg, testlog)
	test.That(t, err, test.ShouldNotBeNil)
	// success
	realSeg := registeredModel{Model: fn, ModelType: RCSegmenter}
	err = segMap.RegisterVisModel(fnName, &realSeg, testlog)
	test.That(t, err, test.ShouldBeNil)
	// segmenter names
	names := segMap.SegmenterNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	creator, err := segMap.modelLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, creator.Model, test.ShouldEqual, fn)
	creator, err = segMap.modelLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such vision model with name")
	test.That(t, creator.Model, test.ShouldBeNil)
	// duplicate
	err = segMap.RegisterVisModel(fnName, &realSeg, testlog)
	test.That(t, err, test.ShouldBeNil)
}

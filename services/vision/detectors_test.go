package vision

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/config"
	inf "go.viam.com/rdk/ml/inference"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func TestDetectorMap(t *testing.T) {
	fn := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	registeredFn := registeredDetector{detector: fn, closer: nil}
	emptyFn := registeredDetector{detector: nil, closer: nil}
	fnName := "x"
	reg := make(detectorMap)
	testlog := golog.NewLogger("testlog")
	// no detector
	err := reg.registerDetector(fnName, &emptyFn, testlog)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot register a nil detector")
	// success
	reg.registerDetector(fnName, &registeredFn, testlog)
	// detector names
	names := reg.detectorNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	det, err := reg.detectorLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, det, test.ShouldEqual, fn)
	det, err = reg.detectorLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Detector with name")
	test.That(t, det, test.ShouldBeNil)
	// duplicate
	err = reg.registerDetector(fnName, &registeredFn, testlog)
	test.That(t, err, test.ShouldBeNil)
	names = reg.detectorNames()
	test.That(t, names, test.ShouldContain, fnName)
	// remove
	err = reg.removeDetector(fnName, testlog)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reg.detectorNames(), test.ShouldNotContain, fnName)
}

func TestDetectorCloser(t *testing.T) {
	fakeDetectFn := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	closer := inf.TFLiteStruct{Info: &inf.TFLiteInfo{100, 100, 3, "uint8", 1, 4, []string{}}}

	d := registeredDetector{detector: fakeDetectFn, closer: &closer}
	reg := make(detectorMap)
	err := reg.registerDetector("x", &d, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	got := reg["x"].closer
	test.That(t, got, test.ShouldNotBeNil)
}

func TestDetectorRemoval(t *testing.T) {
	fakeDetectFn := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	closer, err := addTFLiteModel(artifact.MustPath("vision/tflite/effdet0.tflite"), nil)
	test.That(t, err, test.ShouldBeNil)
	d := registeredDetector{detector: fakeDetectFn, closer: closer}
	testlog := golog.NewTestLogger(t)
	reg := make(detectorMap)
	err = reg.registerDetector("x", &d, testlog)
	test.That(t, err, test.ShouldBeNil)
	err = reg.registerDetector("y", &d, testlog)
	test.That(t, err, test.ShouldBeNil)
	logger, obs := golog.NewObservedTestLogger(t)
	err = reg.removeDetector("z", logger)
	test.That(t, err, test.ShouldBeNil)
	got := obs.All()[len(obs.All())-1].Message
	test.That(t, got, test.ShouldContainSubstring, "no Detector with name")
	err = reg.removeDetector("x", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reg.detectorNames(), test.ShouldNotContain, "x")
}

func TestRegisterTFLiteDetector(t *testing.T) {
	modelLoc := artifact.MustPath("vision/tflite/effdet0.tflite")
	conf := &Attributes{
		DetectorRegistry: []DetectorConfig{
			{
				Name: "my_tflite_det",
				Type: "tflite",
				Parameters: config.AttributeMap{
					"model_path":  modelLoc,
					"label_path":  "",
					"num_threads": 1,
				},
			},
		},
	}
	reg := make(detectorMap)
	err := registerNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
}

func TestRegisterTensorFlowDetector(t *testing.T) {
	conf := &Attributes{
		DetectorRegistry: []DetectorConfig{
			{
				Name:       "my_tensorflow_det",
				Type:       "tensorflow",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(detectorMap)
	err := registerNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, newDetectorTypeNotImplemented("tensorflow"))
}

func TestRegisterColorDetector(t *testing.T) {
	conf := &Attributes{
		DetectorRegistry: []DetectorConfig{
			{
				Name: "my_color_det",
				Type: "color",
				Parameters: config.AttributeMap{
					"segment_size": 150000,
					"tolerance":    0.44,
					"detect_color": "#4F3815",
				},
			},
		},
	}
	reg := make(detectorMap)
	err := registerNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	_, err = reg.detectorLookup("my_color_det")
	test.That(t, err, test.ShouldBeNil)

	// error from bad config
	conf.DetectorRegistry[0].Parameters = nil
	err = registerNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")
}

func TestRegisterUnknownDetector(t *testing.T) {
	conf := &Attributes{
		DetectorRegistry: []DetectorConfig{
			{
				Name:       "my_random_det",
				Type:       "not_real",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(detectorMap)
	err := registerNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, newDetectorTypeNotImplemented("not_real"))
}

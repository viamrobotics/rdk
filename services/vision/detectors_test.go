package vision

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"
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
	reg.removeDetector(fnName, testlog)
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
	reg.removeDetector("z", logger)
	got := obs.All()[len(obs.All())-1].Message
	test.That(t, got, test.ShouldContainSubstring, "no Detector with name")
	reg.removeDetector("x", logger)
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

func TestCloseService(t *testing.T) {
	ctx := context.Background()
	srv := createService(ctx, t, "data/empty.json")
	// success
	cfg := DetectorConfig{
		Name: "test",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#112233",
			"tolerance":    0.4,
			"segment_size": 100,
		},
	}
	err := srv.AddDetector(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	vService := srv.(*visionService)
	fakeStruct := newStruct()
	det := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{}, nil
	}
	registeredFn := registeredDetector{detector: det, closer: fakeStruct}
	logger := golog.NewTestLogger(t)
	err = vService.detReg.registerDetector("fake", &registeredFn, logger)
	test.That(t, err, test.ShouldBeNil)
	err = viamutils.TryClose(ctx, srv)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeStruct.val, test.ShouldEqual, 1)

	detectors, err := srv.GetDetectorNames(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(detectors), test.ShouldEqual, 0)
}

func newStruct() *fakeClosingStruct {
	return &fakeClosingStruct{val: 0}
}

type fakeClosingStruct struct {
	val int
}

func (s *fakeClosingStruct) Close() error {
	s.val++
	return nil
}

func createService(ctx context.Context, t *testing.T, filePath string) Service {
	t.Helper()
	logger := golog.NewTestLogger(t)
	srv, err := New(ctx, nil, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	return srv
}

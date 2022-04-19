package objectdetection

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func TestDetectorRegistry(t *testing.T) {
	fn := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{objdet.NewDetection(image.Rectangle{}, 0.0, "")}, nil
	}
	fnName := "x"
	reg := make(detRegistry)
	// no detector
	err := reg.RegisterDetector(context.Background(), fnName, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot register a nil detector")
	// success
	reg.RegisterDetector(context.Background(), fnName, fn)
	// detector names
	names := reg.DetectorNames()
	test.That(t, names, test.ShouldNotBeNil)
	test.That(t, names, test.ShouldContain, fnName)
	// look up
	det, err := reg.DetectorLookup(fnName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, det, test.ShouldEqual, fn)
	det, err = reg.DetectorLookup("z")
	test.That(t, err.Error(), test.ShouldContainSubstring, "no Detector with name")
	test.That(t, det, test.ShouldBeNil)
	// duplicate
	err = reg.RegisterDetector(context.Background(), fnName, fn)
	test.That(t, err.Error(), test.ShouldContainSubstring, "trying to register two detectors with the same name")
}

func TestRegisterTFLiteDetector(t *testing.T) {
	conf := &Attributes{
		Registry: []RegistryConfig{
			{
				Name:       "my_tflite_det",
				Type:       "tflite",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(detRegistry)
	err := RegisterNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, NewDetectorTypeNotImplemented("tflite"))
}

func TestRegisterTensorFlowDetector(t *testing.T) {
	conf := &Attributes{
		Registry: []RegistryConfig{
			{
				Name:       "my_tensorflow_det",
				Type:       "tensorflow",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(detRegistry)
	err := RegisterNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, NewDetectorTypeNotImplemented("tensorflow"))
}

func TestRegisterColorDetector(t *testing.T) {
	conf := &Attributes{
		Registry: []RegistryConfig{
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
	reg := make(detRegistry)
	err := RegisterNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	_, err = reg.DetectorLookup("my_color_det")
	test.That(t, err, test.ShouldBeNil)

	// error from bad config
	conf.Registry[0].Parameters = nil
	err = RegisterNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")
}

func TestRegisterUnknownDetector(t *testing.T) {
	conf := &Attributes{
		Registry: []RegistryConfig{
			{
				Name:       "my_random_det",
				Type:       "not_real",
				Parameters: config.AttributeMap{},
			},
		},
	}
	reg := make(detRegistry)
	err := RegisterNewDetectors(context.Background(), reg, conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, NewDetectorTypeNotImplemented("not_real"))
}

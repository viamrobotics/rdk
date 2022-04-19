package objectdetection

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

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
	err := registerNewDetectors(conf, golog.NewTestLogger(t))
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
	err := registerNewDetectors(conf, golog.NewTestLogger(t))
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
	err := registerNewDetectors(conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	_, err = DetectorLookup("my_color_det")
	test.That(t, err, test.ShouldBeNil)

	// error from bad config
	conf.Registry[0].Parameters = nil
	err = registerNewDetectors(conf, golog.NewTestLogger(t))
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
	err := registerNewDetectors(conf, golog.NewTestLogger(t))
	test.That(t, err, test.ShouldBeError, NewDetectorTypeNotImplemented("not_real"))
}

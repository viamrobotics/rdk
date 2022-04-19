package objectdetection

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
)

// DetectorType defines what detector types are known.
type DetectorType string

// The set of allowed detector types.
const (
	TFLiteType     = DetectorType("tflite")
	TensorFlowType = DetectorType("tensorflow")
	ColorType      = DetectorType("color")
)

// NewDetectorTypeNotImplemented is used when the detector type is not implemented.
func NewDetectorTypeNotImplemented(name string) error {
	return errors.Errorf("detector type %q is not implemented", name)
}

// Config contains a list of the user-provided details necessary to register a new detector.
type Config struct {
	Registry []DetectorRegistryConfig `json:"detector_registry"`
}

// DetectorRegistryConfig specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type DetectorRegistryConfig struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Parameters config.AttributeMap `json:"parameters"`
}

// registerNewDetectors take a Config and parses each element by type to create an RDK Detector
// and register it to the detector registry.
func registerNewDetectors(attrs *Config, logger golog.Logger) error {
	for _, attr := range attrs.Registry {
		logger.Debugf("adding detector %q of type %s", attr.Name, attr.Type)
		switch DetectorType(attr.Type) {
		case TFLiteType:
			return NewDetectorTypeNotImplemented(attr.Type)
		case TensorFlowType:
			return NewDetectorTypeNotImplemented(attr.Type)
		case ColorType:
			err := registerColorDetector(&attr)
			if err != nil {
				return err
			}
		default:
			return NewDetectorTypeNotImplemented(attr.Type)
		}
	}
	return nil
}

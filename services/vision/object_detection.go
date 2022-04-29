package vision

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

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

// newDetectorTypeNotImplemented is used when the detector type is not implemented.
func newDetectorTypeNotImplemented(name string) error {
	return errors.Errorf("detector type %q is not implemented", name)
}

// DetectorConfig specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type DetectorConfig struct {
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Parameters config.AttributeMap `json:"parameters"`
}

// registerNewDetectors take an attributes struct and parses each element by type to create an RDK Detector
// and register it to the detector map.
func registerNewDetectors(ctx context.Context, dm detectorMap, attrs *Attributes, logger golog.Logger) error {
	_, span := trace.StartSpan(ctx, "service::vision::registerNewDetectors")
	defer span.End()
	for _, attr := range attrs.DetectorRegistry {
		logger.Debugf("adding detector %q of type %s", attr.Name, attr.Type)
		switch DetectorType(attr.Type) {
		case TFLiteType:
			return newDetectorTypeNotImplemented(attr.Type)
		case TensorFlowType:
			return newDetectorTypeNotImplemented(attr.Type)
		case ColorType:
			err := registerColorDetector(dm, &attr)
			if err != nil {
				return err
			}
		default:
			return newDetectorTypeNotImplemented(attr.Type)
		}
	}
	return nil
}

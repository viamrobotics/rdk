package vision

import (
	"context"
	"io"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	objdet "go.viam.com/rdk/vision/objectdetection"
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

// detectorMap stores the registered detectors of the service.
type detectorMap map[string]registeredDetector

type registeredDetector struct {
	detector objdet.Detector
	closer   io.Closer
}

// registerDetector registers a Detector type to a registry.
func (dm detectorMap) registerDetector(name string, det *registeredDetector, logger golog.Logger) error {
	if det == nil || det.detector == nil {
		return errors.Errorf("cannot register a nil detector: %s", name)
	}
	if _, old := dm[name]; old {
		logger.Infof("overwriting the detector with name: %s", name)
	}

	if det.closer != nil {
		dm[name] = registeredDetector{detector: det.detector, closer: det.closer}
		return nil
	}
	dm[name] = registeredDetector{detector: det.detector, closer: nil}
	return nil
}

// detectorLookup looks up a detector by name. An error is returned if
// there is no detector by that name.
func (dm detectorMap) detectorLookup(name string) (objdet.Detector, error) {
	det, ok := dm[name]
	if ok {
		return det.detector, nil
	}
	return nil, errors.Errorf("no Detector with name %q", name)
}

// detectorNames returns a slice of all the segmenter names in the registry.
func (dm detectorMap) detectorNames() []string {
	names := make([]string, 0, len(dm))
	for name := range dm {
		names = append(names, name)
	}
	return names
}

// removeDetector closes the model and removes the detector from the registry.
func (dm detectorMap) removeDetector(name string, logger golog.Logger) error {
	if _, ok := dm[name]; !ok {
		logger.Infof("no Detector with name %s", name)
		return nil
	}

	if dm[name].closer != nil {
		err := dm[name].closer.Close()
		if err != nil {
			return err
		}
	}
	delete(dm, name)

	return nil
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
			return registerTfliteDetector(ctx, dm, &attr, logger)
		case TensorFlowType:
			return newDetectorTypeNotImplemented(attr.Type)
		case ColorType:
			err := registerColorDetector(ctx, dm, &attr, logger)
			if err != nil {
				return err
			}
		default:
			return newDetectorTypeNotImplemented(attr.Type)
		}
	}
	return nil
}

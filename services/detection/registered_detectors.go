// Package detection is the service that allows you to access registered detectors and cameras
// and return bounding boxes and streams of detections. Also allows you to register new
// object detectors.
package detection

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/vision/objectdetection"
)

// The detector registry.
var detectorRegistry = make(map[string]objectdetection.Detector)

// RegisterDetector registers a Detector type to a registration.
func RegisterDetector(name string, det objectdetection.Detector) {
	if _, old := detectorRegistry[name]; old {
		panic(errors.Errorf("trying to register two detectors with the same name: %s", name))
	}
	if det == nil {
		panic(errors.Errorf("cannot register a nil detector: %s", name))
	}
	detectorRegistry[name] = det
}

// DetectorLookup looks up a detector registration by name. An error is returned if
// there is no registration.
func DetectorLookup(name string) (objectdetection.Detector, error) {
	registration, ok := RegisteredDetectors()[name]
	if ok {
		return registration, nil
	}
	return nil, errors.Errorf("no Detector with name %q", name)
}

// RegisteredDetectors returns a copy of the registered detectors.
func RegisteredDetectors() map[string]objectdetection.Detector {
	copied, err := copystructure.Copy(detectorRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]objectdetection.Detector)
}

// DetectorNames returns a slice of all the segmenter names in the registry.
func DetectorNames() []string {
	names := make([]string, 0, len(detectorRegistry))
	for name := range detectorRegistry {
		names = append(names, name)
	}
	return names
}

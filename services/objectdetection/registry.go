package objectdetection

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	objdet "go.viam.com/rdk/vision/objectdetection"
)

// The detector registry.
var detectorRegistry = make(map[string]objdet.Detector)

// RegisterDetector registers a Detector type to a registration.
func RegisterDetector(name string, det objdet.Detector) {
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
func DetectorLookup(name string) (objdet.Detector, error) {
	registration, ok := RegisteredDetectors()[name]
	if ok {
		return registration, nil
	}
	return nil, errors.Errorf("no Detector with name %q", name)
}

// RegisteredDetectors returns a copy of the registered detectors.
func RegisteredDetectors() map[string]objdet.Detector {
	copied, err := copystructure.Copy(detectorRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]objdet.Detector)
}

// DetectorNames returns a slice of all the segmenter names in the registry.
func DetectorNames() []string {
	names := make([]string, 0, len(detectorRegistry))
	for name := range detectorRegistry {
		names = append(names, name)
	}
	return names
}

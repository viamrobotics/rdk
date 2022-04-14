package detectionservice

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

// A DetectorRegistration stores both the Detection function and the form of the parameters it takes as an argument.
type DetectorRegistration struct {
	objectdetection.Detector
	Parameters []utils.TypedName
}

// The detector registry.
var detectorRegistry = make(map[string]DetectorRegistration)

// RegisterDetector registers a Detector type to a registration.
func RegisterDetector(name string, det DetectorRegistration) {
	if _, old := detectorRegistry[name]; old {
		panic(errors.Errorf("trying to register two detectors with the same name: %s", name))
	}
	if det.Detector == nil {
		panic(errors.Errorf("cannot register a nil detector: %s", name))
	}
	detectorRegistry[name] = det
}

// DetectorLookup looks up a detector registration by name. An error is returned if
// there is no registration.
func DetectorLookup(name string) (*DetectorRegistration, error) {
	registration, ok := RegisteredDetectors()[name]
	if ok {
		return &registration, nil
	}
	return nil, errors.Errorf("no Detector with name %q", name)
}

// RegisteredDetectors returns a copy of the registered detectors.
func RegisteredDetectors() map[string]DetectorRegistration {
	copied, err := copystructure.Copy(detectorRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]DetectorRegistration)
}

// DetectorNames returns a slice of all the segmenter names in the registry.
func DetectorNames() []string {
	names := make([]string, 0, len(detectorRegistry))
	for name := range detectorRegistry {
		names = append(names, name)
	}
	return names
}

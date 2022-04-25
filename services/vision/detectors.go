package vision

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	objdet "go.viam.com/rdk/vision/objectdetection"
)

// detectorMap stores the registered detectors of the service.
type detectorMap map[string]objdet.Detector

// registerDetector registers a Detector type to a registry.
func (dm detectorMap) registerDetector(name string, det objdet.Detector) error {
	if _, old := dm[name]; old {
		return errors.Errorf("trying to register two detectors with the same name: %s", name)
	}
	if det == nil {
		return errors.Errorf("cannot register a nil detector: %s", name)
	}
	dm[name] = det
	return nil
}

// detectorLookup looks up a detector by name. An error is returned if
// there is no detector by that name.
func (dm detectorMap) detectorLookup(name string) (objdet.Detector, error) {
	det, ok := dm.registeredDetectors()[name]
	if ok {
		return det, nil
	}
	return nil, errors.Errorf("no Detector with name %q", name)
}

// registeredDetectors returns a copy of the registered detectors.
func (dm detectorMap) registeredDetectors() detectorMap {
	copied, err := copystructure.Copy(dm)
	if err != nil {
		panic(err)
	}
	return copied.(detectorMap)
}

// detectorNames returns a slice of all the segmenter names in the registry.
func (dm detectorMap) detectorNames() []string {
	names := make([]string, 0, len(dm))
	for name := range dm {
		names = append(names, name)
	}
	return names
}

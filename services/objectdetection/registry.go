package objectdetection

import (
	"context"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	objdet "go.viam.com/rdk/vision/objectdetection"
)

// detRegistry is a detector registry.
type detRegistry map[string]objdet.Detector

// RegisterDetector registers a Detector type to a registry.
func (r detRegistry) RegisterDetector(ctx context.Context, name string, det objdet.Detector) error {
	if _, old := r[name]; old {
		return errors.Errorf("trying to register two detectors with the same name: %s", name)
	}
	if det == nil {
		return errors.Errorf("cannot register a nil detector: %s", name)
	}
	r[name] = det
	return nil
}

// DetectorLookup looks up a detector registration by name. An error is returned if
// there is no registration.
func (r detRegistry) DetectorLookup(name string) (objdet.Detector, error) {
	registration, ok := r.RegisteredDetectors()[name]
	if ok {
		return registration, nil
	}
	return nil, errors.Errorf("no Detector with name %q", name)
}

// RegisteredDetectors returns a copy of the registered detectors.
func (r detRegistry) RegisteredDetectors() detRegistry {
	copied, err := copystructure.Copy(r)
	if err != nil {
		panic(err)
	}
	return copied.(detRegistry)
}

// DetectorNames returns a slice of all the segmenter names in the registry.
func (r detRegistry) DetectorNames() []string {
	names := make([]string, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	return names
}

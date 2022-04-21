package objectsegmentation

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

// A SegmenterRegistration stores both the Segmenter function and the form of the parameters it takes as an argument.
type SegmenterRegistration struct {
	segmentation.Segmenter
	Parameters []utils.TypedName
}

// The segmenter map.
type segmenterMap map[string]SegmenterRegistration

// registerSegmenter registers a Segmenter type to a registration.
func (sm segmenterMap) registerSegmenter(name string, seg SegmenterRegistration) error {
	if _, old := sm[name]; old {
		return errors.Errorf("trying to register two segmenters with the same name: %s", name)
	}
	if seg.Segmenter == nil {
		return errors.Errorf("cannot register a nil segmenter: %s", name)
	}
	sm[name] = seg
	return nil
}

// segmenterLookup looks up a segmenter registration by name. An error is returned if
// there is no registration.
func (sm segmenterMap) segmenterLookup(name string) (*SegmenterRegistration, error) {
	registration, ok := sm.registeredSegmenters()[name]
	if ok {
		return &registration, nil
	}
	return nil, errors.Errorf("no Segmenter with name %q", name)
}

// registeredSegmenters returns a copy of the registered segmenters.
func (sm segmenterMap) registeredSegmenters() map[string]SegmenterRegistration {
	copied, err := copystructure.Copy(sm)
	if err != nil {
		panic(err)
	}
	return copied.(segmenterMap)
}

// segmenterNames returns a slice of all the segmenter names in the map.
func (sm segmenterMap) segmenterNames() []string {
	names := make([]string, 0, len(sm))
	for name := range sm {
		names = append(names, name)
	}
	return names
}

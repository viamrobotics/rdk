package vision

import (
	"github.com/edaniels/golog"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

// RadiusClusteringSegmenter is  the name of a segmenter that finds well separated objects on a flat plane.
const RadiusClusteringSegmenter = "radius_clustering"

// A SegmenterRegistration stores both the Segmenter function and the form of the parameters it takes as an argument.
type SegmenterRegistration struct {
	segmentation.Segmenter
	Parameters []utils.TypedName
}

// The segmenter map.
type segmenterMap map[string]SegmenterRegistration

// registerSegmenter registers a Segmenter type to a registration.
func (sm segmenterMap) registerSegmenter(name string, seg SegmenterRegistration, logger golog.Logger) error {
	if _, old := sm[name]; old {
		logger.Infof("overwriting the segmenter with name: %s", name)
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
func (sm segmenterMap) registeredSegmenters() segmenterMap {
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

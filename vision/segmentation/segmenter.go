package segmentation

import (
	"context"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// A Segmenter is a function that takes images/pointclouds from an input camera and segments them into objects.
type Segmenter func(ctx context.Context, c camera.Camera, parameters config.AttributeMap) ([]*vision.Object, error)

// A Registration stores both the Segmenter function and the form of the parameters it takes as an argument.
type Registration struct {
	Segmenter
	Parameters []utils.TypedName
}

// The segmenter registry.
var segmenterRegistry = make(map[string]Registration)

// RegisterSegmenter registers a Segmenter type to a registration.
func RegisterSegmenter(name string, seg Registration) {
	if _, old := segmenterRegistry[name]; old {
		panic(errors.Errorf("trying to register two segmenters with the same name: %s", name))
	}
	if seg.Segmenter == nil {
		panic(errors.Errorf("cannot register a nil segmenter: %s", name))
	}
	segmenterRegistry[name] = seg
}

// SegmenterLookup looks up a segmenter registration by name. An error is returned if
// there is no registration.
func SegmenterLookup(name string) (*Registration, error) {
	registration, ok := RegisteredSegmenters()[name]
	if ok {
		return &registration, nil
	}
	return nil, errors.Errorf("no Segmenter with name %q", name)
}

// RegisteredSegmenters returns a copy of the registered segmenters.
func RegisteredSegmenters() map[string]Registration {
	copied, err := copystructure.Copy(segmenterRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Registration)
}

// SegmenterNames returns a slice of all the segmenter names in the registry.
func SegmenterNames() []string {
	names := make([]string, 0, len(segmenterRegistry))
	for name := range segmenterRegistry {
		names = append(names, name)
	}
	return names
}

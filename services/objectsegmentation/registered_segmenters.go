package objectsegmentation

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

// ColorObjectsSegmenter is the name of a segmenter that finds objects using the bounding boxes of a color detector.
const ColorObjectsSegmenter = "color_objects"

// RadiusClusteringSegmenter is  the name of a segmenter that finds well separated objects on a flat plane.
const RadiusClusteringSegmenter = "radius_clustering"

func init() {
	RegisterSegmenter(ColorObjectsSegmenter,
		SegmenterRegistration{
			segmentation.Segmenter(segmentation.ColorObjects),
			utils.JSONTags(segmentation.ColorObjectsConfig{}),
		})

	RegisterSegmenter(RadiusClusteringSegmenter,
		SegmenterRegistration{
			segmentation.Segmenter(segmentation.RadiusClustering),
			utils.JSONTags(segmentation.RadiusClusteringConfig{}),
		})
}

// A SegmenterRegistration stores both the Segmenter function and the form of the parameters it takes as an argument.
type SegmenterRegistration struct {
	segmentation.Segmenter
	Parameters []utils.TypedName
}

// The segmenter registry.
var segmenterRegistry = make(map[string]SegmenterRegistration)

// RegisterSegmenter registers a Segmenter type to a registration.
func RegisterSegmenter(name string, seg SegmenterRegistration) {
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
func SegmenterLookup(name string) (*SegmenterRegistration, error) {
	registration, ok := RegisteredSegmenters()[name]
	if ok {
		return &registration, nil
	}
	return nil, errors.Errorf("no Segmenter with name %q", name)
}

// RegisteredSegmenters returns a copy of the registered segmenters.
func RegisteredSegmenters() map[string]SegmenterRegistration {
	copied, err := copystructure.Copy(segmenterRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]SegmenterRegistration)
}

// SegmenterNames returns a slice of all the segmenter names in the registry.
func SegmenterNames() []string {
	names := make([]string, 0, len(segmenterRegistry))
	for name := range segmenterRegistry {
		names = append(names, name)
	}
	return names
}

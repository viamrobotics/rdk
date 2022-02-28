package registry

import (
	"context"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
)

type (
	// A CreateSegmenter creates a service from a given config.
	CreateSegmenter func(ctx context.Context) (interface{}, error)
)

// Segmenter registry.
var segmenterRegistry = make(map[string]Segmenter)

// Segmenter stores a Segmenter constructor (mandatory).
type Segmenter struct {
	RegDebugInfo
	Constructor CreateSegmenter
}

// RegisterSegmenter registers a Segmenter type to a registration.
func RegisterSegmenter(name string, creator Segmenter) {
	creator.RegistrarLoc = getCallerName()
	if _, old := segmenterRegistry[name]; old {
		panic(errors.Errorf("trying to register two segmenters with the same name: %s", name))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for segmenter: %s", name))
	}
	segmenterRegistry[name] = creator
}

// SegmenterLookup looks up a segmenter registration by name. nil is returned if
// there is no registration.
func SegmenterLookup(name string) *Segmenter {
	registration, ok := RegisteredSegmenters()[name]
	if ok {
		return &registration
	}
	return nil
}

// RegisteredSegmenters returns a copy of the registered segmenters.
func RegisteredSegmenters() map[string]Segmenter {
	copied, err := copystructure.Copy(segmenterRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Segmenter)
}

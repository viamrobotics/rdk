// Package discovery implements a discovery service.
package discovery

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

type (
	// Discover tries to find potential configurations for a component. The return type
	// is expected to be comprised of string keys (or it should be possible to decompose
	// it into string keys) and values comprised of primitives, list of primitives, maps
	// with string keys (or at least can be decomposed into one), or lists of the
	// aforementioned type of maps. Results with other types of data are not guaranteed.
	Discover func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error)
)

var discoveryFunctions = map[Key]Discover{}

func DiscoveryFunctionLookup(subtypeName resource.SubtypeName, model string) (Discover, bool) {
	if _, ok := lookupSubtype(subtypeName); !ok {
		return nil, false
	}
	key := Key{subtypeName, model}
	df, ok := discoveryFunctions[key]
	return df, ok
}

func RegisterDiscoveryFunction(subtypeName resource.SubtypeName, model string, discover Discover) {
	if _, ok := lookupSubtype(subtypeName); !ok {
		panic(errors.Errorf("trying to register discovery function for unregistered subtype %q.", subtypeName))
	}
	key := Key{subtypeName, model}
	if _, ok := discoveryFunctions[key]; ok {
		panic(errors.Errorf("trying to register two discovery functions for subtype %q and model %q.", subtypeName, model))
	}
	discoveryFunctions[key] = discover
}

func lookupSubtype(subtypeName resource.SubtypeName) (*resource.Subtype, bool) {
	for s := range registry.RegisteredResourceSubtypes() {
		if s.ResourceSubtype == subtypeName {
			return &s, true
		}
	}
	return nil, false
}

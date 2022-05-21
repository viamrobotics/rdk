// Package discovery implements types to support robot component discovery.
package discovery

import (
	"context"
	"fmt"

	"go.viam.com/rdk/resource"
)

type (
	// Key is a tuple of subtype name and model used to lookup discovery functions.
	Key struct {
		SubtypeName resource.SubtypeName
		Model       string
	}

	// Discovery holds a resource name and its corresponding discovery. Discovery is
	// expected to be comprised of string keys and values comprised of primitives, list
	// of primitives, maps with string keys (or at least can be decomposed into one), or
	// lists of the forementioned type of maps. Results with other types of data are not
	// guaranteed.
	Discovery struct {
		Key        Key
		Discovered interface{}
	}

	// Discover tries to find potential configurations for a component. The return type
	// is expected to be comprised of string keys (or it should be possible to decompose
	// it into string keys) and values comprised of primitives, list of primitives, maps
	// with string keys (or at least can be decomposed into one), or lists of the
	// aforementioned type of maps. Results with other types of data are not guaranteed.
	Function func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error)

	// DiscoverError indicates that a Discover function has returned an error.
	DiscoverError struct {
		Key Key
	}
)

func (e *DiscoverError) Error() string {
	return fmt.Sprintf("failed to get discovery for subtype %q and model %q", e.Key.SubtypeName, e.Key.Model)
}

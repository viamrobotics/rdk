// Package discovery implements types to support robot component discovery.
package discovery

import (
	"context"
	"fmt"

	"go.viam.com/rdk/resource"
)

type (
	// Query is a tuple of subtype name and model used to lookup discovery functions.
	Query struct {
		SubtypeName resource.SubtypeName
		Model       string
	}

	// Discovery holds a Query and a corresponding discovered component configuration.
	// A discovered component configuration can be comprised primitives, list of
	// primitives, maps with string keys (or at least can be decomposed into one), or
	// lists of the forementioned type of maps. Results with other types of data are not
	// guaranteed.
	Discovery struct {
		Query      Query
		Discovered interface{}
	}

	// A Function that returns a discovery.
	Function func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error)

	// DiscoverError indicates that a Discover function has returned an error.
	DiscoverError struct {
		Query Query
	}
)

func (e *DiscoverError) Error() string {
	return fmt.Sprintf("failed to get discovery for subtype %q and model %q", e.Query.SubtypeName, e.Query.Model)
}

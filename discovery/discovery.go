// Package discovery implements types to support robot component discovery.
package discovery

import (
	"context"
	"fmt"

	"go.viam.com/rdk/resource"
)

type (
	// Query is a tuple of subtype (api) and model used to lookup discovery functions.
	Query struct {
		API   resource.Subtype
		Model resource.Model
	}

	// Discover is a function that discovers component configurations.
	Discover func(ctx context.Context) (interface{}, error)

	// Discovery holds a Query and a corresponding discovered component configuration. A
	// discovered component configuration can be comprised of primitives, a list of
	// primitives, maps with string keys (or at least can be decomposed into one), or
	// lists of the forementioned type of maps. Results with other types of data are not
	// guaranteed.
	Discovery struct {
		Query   Query
		Results interface{}
	}

	// DiscoverError indicates that a Discover function has returned an error.
	DiscoverError struct {
		Query Query
	}
)

func (e *DiscoverError) Error() string {
	return fmt.Sprintf("failed to get discovery for subtype %q and model %q", e.Query.API, e.Query.Model)
}

// NewQuery returns a discovery query for a given subtype and model.
func NewQuery(subtype resource.Subtype, model resource.Model) Query {
	return Query{subtype, model}
}

package resource

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
)

type (
	// DiscoveryQuery is a tuple of subtype (api) and model used to lookup discovery functions.
	DiscoveryQuery struct {
		API   Subtype
		Model Model
	}

	// DiscoveryFunc is a function that discovers component configurations.
	DiscoveryFunc func(ctx context.Context, logger golog.Logger) (interface{}, error)

	// Discovery holds a Query and a corresponding discovered component configuration. A
	// discovered component configuration can be comprised of primitives, a list of
	// primitives, maps with string keys (or at least can be decomposed into one), or
	// lists of the forementioned type of maps. Results with other types of data are not
	// guaranteed.
	Discovery struct {
		Query   DiscoveryQuery
		Results interface{}
	}

	// DiscoverError indicates that a Discover function has returned an error.
	DiscoverError struct {
		Query DiscoveryQuery
	}
)

func (e *DiscoverError) Error() string {
	return fmt.Sprintf("failed to get discovery for subtype %q and model %q", e.Query.API, e.Query.Model)
}

// NewDiscoveryQuery returns a discovery query for a given subtype and model.
func NewDiscoveryQuery(subtype Subtype, model Model) DiscoveryQuery {
	return DiscoveryQuery{subtype, model}
}

// Package discovery implements functions to find robot components.
package discovery

import (
	"context"
	"fmt"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
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

	// DiscoverError indicates that a Discover function has returned an error.
	DiscoverError struct {
		Key Key
	}
)

func (e *DiscoverError) Error() string {
	return fmt.Sprintf("failed to get discovery for subtype %q and model %q", e.Key.SubtypeName, e.Key.Model)
}

// Discover takes a list of subtype and model name pairs and returns their corresponding
// discoveries.
func Discover(ctx context.Context, keys []Key) ([]Discovery, error) {
	// dedupe keys
	deduped := make(map[Key]struct{}, len(keys))
	for _, k := range keys {
		deduped[k] = struct{}{}
	}

	discoveries := make([]Discovery, 0, len(deduped))
	for key := range deduped {
		discoveryFunction, ok := FunctionLookup(key.SubtypeName, key.Model)
		if !ok {
			rlog.Logger.Warnw("no discovery function registered", "subtype", key.SubtypeName, "model", key.Model)
			continue
		}

		if discoveryFunction != nil {
			discovered, err := discoveryFunction(ctx, key.SubtypeName, key.Model)
			if err != nil {
				return nil, &DiscoverError{key}
			}
			discoveries = append(discoveries, Discovery{Key: key, Discovered: discovered})
		}
	}
	return discoveries, nil
}

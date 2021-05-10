package tools

import (
	"go.viam.com/robotcore/artifact"
)

// Export exports any artifacts not present in global cache tree
// to the underlying store of the cache.
func Export() error {
	cache, err := artifact.GlobalCache()
	if err != nil {
		return err
	}

	return cache.WriteThroughUser()
}

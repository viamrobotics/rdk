package tools

import (
	"go.viam.com/core/artifact"
)

// Remove removes the given path from the tree and root if present
// but not from the cache. Use clean --cache to clear out the cache.
func Remove(filePath string) error {
	cache, err := artifact.GlobalCache()
	if err != nil {
		return err
	}

	return cache.Remove(filePath)
}

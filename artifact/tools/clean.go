package tools

import (
	"go.viam.com/robotcore/artifact"
)

// Clean cleans the global cache of any artifacts not present in the tree.
func Clean() error {
	cache, err := artifact.GlobalCache()
	if err != nil {
		return err
	}

	return cache.Clean()
}

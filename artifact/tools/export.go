package tools

import (
	"go.viam.com/robotcore/artifact"
)

func Export() error {
	cache, err := artifact.GlobalCache()
	if err != nil {
		return err
	}

	return cache.WriteThroughUser()
}

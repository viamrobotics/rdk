package tools

import "go.viam.com/robotcore/artifact"

func Import() error {
	cache, err := artifact.GlobalCache()
	if err != nil {
		return err
	}

	_, err = cache.Ensure("/")
	return err
}

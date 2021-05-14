// Package artifact contains a solution for storing and fetching versioned blobs of data
// that are resolved on demand.
//
// By default, a configuration file called .artifact.json is searched for up the directory
// roots. It defines where to store resolved files as well as where to fetch blobs from.
// In addition, when unspecified, versioned blobs are stored alongside the configuration file
// in .artifact.tree.json which is a directory tree mapping files to content addresses. The
// system uses concepts from content-addressable storage in order to simplify storage and
// offer deduplication.
package artifact

import "github.com/go-errors/errors"

// Path returns the local file system path to the given artifact path. It
// errors if it does not exist or cannot be ensured to exist.
func Path(to string) (string, error) {
	cache, err := GlobalCache()
	if err != nil {
		return "", err
	}
	actualPath, err := cache.Ensure(to, true)
	if err != nil {
		return "", errors.Errorf("error ensuring %q: %w", to, err)
	}
	return actualPath, nil
}

// MustPath works like Path but panics if any error occurs. Useful in tests.
func MustPath(to string) string {
	resolved, err := Path(to)
	if err != nil {
		panic(err)
	}
	return resolved
}

// NewPath returns the would be path to an artifact on the local file system.
func NewPath(to string) (string, error) {
	cache, err := GlobalCache()
	if err != nil {
		return "", err
	}
	return cache.NewPath(to), nil
}

// MustNewPath works like NewPath but panics if any error occurs. Useful in tests.
func MustNewPath(to string) string {
	resolved, err := NewPath(to)
	if err != nil {
		panic(err)
	}
	return resolved
}

// Package artifact contains a solution for storing and fetching versioned blobs of data
// that are resolved on demand.
//
// By default, a configuration directory called .artifact is searched for up the directory
// roots. It looks for a file called config.json in it. It defines where to store resolved
// files as well as where to fetch blobs from. In addition, when unspecified, versioned
// blobs are stored alongside the configuration file in tree.json which is a
// directory tree mapping files to content addresses. The system uses concepts from
// content-addressable storage in order to simplify storage and offer deduplication.
package artifact

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/utils"
)

// Path returns the local file system path to the given artifact path. It
// errors if it does not exist or cannot be ensured to exist.
func Path(to string) (string, error) {
	cache, err := GlobalCache()
	if err != nil {
		return "", err
	}
	actualPath, err := cache.Ensure(to, true)
	if err != nil {
		return "", errors.Wrapf(err, "error ensuring %q", to)
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

// emplaceFile ensures that a given artifact identified by a given hash
// is placed in the given path (creating parent directories along the way).
func emplaceFile(store Store, hash, path string) error {
	if err := store.Contains(hash); err != nil {
		return err
	}

	//nolint:gosec
	if existing, err := os.Open(path); err == nil {
		data, err := io.ReadAll(existing)
		if err != nil {
			return multierr.Combine(err, existing.Close())
		}
		if err := existing.Close(); err != nil {
			return err
		}
		existingHash, err := computeHash(data)
		if err != nil {
			return err
		}
		if existingHash == hash {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return errors.Errorf("error removing old artifact at %q", path)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	hashFile, err := store.Load(hash)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, hashFile.Close())
	}()

	tempFile, err := os.CreateTemp(filepath.Dir(path), hash)
	if err != nil {
		return err
	}
	var successful bool
	defer func() {
		if !successful {
			utils.UncheckedError(os.Remove(tempFile.Name()))
		}
	}()
	if err := os.Chmod(tempFile.Name(), 0o600); err != nil {
		return err
	}
	if _, err = io.Copy(tempFile, hashFile); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempFile.Name(), path); err != nil {
		return err
	}
	successful = true
	return nil
}

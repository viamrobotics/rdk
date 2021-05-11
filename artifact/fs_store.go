package artifact

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"go.uber.org/multierr"
)

// newFileSystemStore returns a new fileSystemStore based on the given config.
func newFileSystemStore(config *fileSystemStoreConfig) (*fileSystemStore, error) {
	dirStat, err := os.Stat(config.Path)
	if err != nil {
		return nil, err
	}
	if !dirStat.IsDir() {
		return nil, fmt.Errorf("expected path to be directory %q", config.Path)
	}
	return &fileSystemStore{dir: config.Path}, nil
}

// A fileSystemStore stores artifacts in a local file system. It generally
// stores artifacts by their node hash but can also emplace those same files
// in a directory structure.
type fileSystemStore struct {
	dir string
}

func (s *fileSystemStore) Contains(hash string) error {
	fileInfos, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, info := range fileInfos {
		if info.Name() == hash {
			return nil
		}
	}
	return NewErrArtifactNotFoundHash(hash)
}

func (s *fileSystemStore) pathToHashFile(hash string) string {
	return filepath.Join(s.dir, hash)
}

func (s *fileSystemStore) Load(hash string) (io.ReadCloser, error) {
	if err := s.Contains(hash); err != nil {
		return nil, err
	}
	return os.Open(s.pathToHashFile(hash))
}

func (s *fileSystemStore) Store(hash string, r io.Reader) (err error) {
	path := s.pathToHashFile(hash)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = multierr.Combine(err, f.Close(), os.Remove(path))
		} else {
			err = f.Close()
		}
	}()
	_, err = io.Copy(f, r)
	return
}

// Emplace ensures that a given artifact identified by a given hash
// is placed in the given path (creating parent directories along the way).
func (s *fileSystemStore) Emplace(hash, path string) (err error) {
	if err := s.Contains(hash); err != nil {
		return err
	}
	pointsTo, err := os.Readlink(path)
	if err == nil {
		if filepath.Base(pointsTo) == hash {
			return nil
		}
	}

	if existing, err := os.Open(path); err == nil {
		data, err := ioutil.ReadAll(existing)
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
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error removing old artifact at %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	hashFile, err := os.Open(s.pathToHashFile(hash))
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, hashFile.Close())
	}()

	emplacedFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err = multierr.Combine(err, os.Remove(path))
		} else {
			err = emplacedFile.Close()
		}
	}()
	_, err = io.Copy(emplacedFile, hashFile)
	return
}

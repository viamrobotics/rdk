package artifact

import (
	"io"
	"os"
	"path/filepath"

	"github.com/go-errors/errors"
	"go.uber.org/multierr"
)

// newFileSystemStore returns a new fileSystemStore based on the given config.
func newFileSystemStore(config *fileSystemStoreConfig) (*fileSystemStore, error) {
	dirStat, err := os.Stat(config.Path)
	if err != nil {
		return nil, err
	}
	if !dirStat.IsDir() {
		return nil, errors.Errorf("expected path to be directory %q", config.Path)
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

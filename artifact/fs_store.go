package artifact

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/multierr"
)

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
			err = multierr.Combine(err, os.Remove(path))
		} else {
			err = f.Close()
		}
	}()
	_, err = io.Copy(f, r)
	return
}

func (s *fileSystemStore) Emplace(hash, path string) error {
	if err := s.Contains(hash); err != nil {
		return err
	}
	pointsTo, err := os.Readlink(path)
	if err == nil {
		if filepath.Base(pointsTo) == hash {
			return nil
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error removing old artifact at %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.Symlink(s.pathToHashFile(hash), path)
}

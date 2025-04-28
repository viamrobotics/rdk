package artifact

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"go.viam.com/utils"
)

// newFileSystemStore returns a new fileSystemStore based on the given config.
func newFileSystemStore(config *FileSystemStoreConfig) (*fileSystemStore, error) {
	dirStat, err := os.Stat(config.Path)
	if err == nil && !dirStat.IsDir() {
		return nil, errors.Errorf("expected path to be directory %q", config.Path)
	} else if err != nil {
		if err := os.MkdirAll(config.Path, 0o750); err != nil {
			return nil, err
		}
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
	return NewArtifactNotFoundHashError(hash)
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

// AtomicStore writes reader contents to a temp file and then renames to
// path, ensuring safer, atomic file writes.
func AtomicStore(path string, r io.Reader, hash string) (err error) {
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
	if _, err = io.Copy(tempFile, r); err != nil {
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

func (s *fileSystemStore) Store(hash string, r io.Reader) (err error) {
	path := s.pathToHashFile(hash)

	return AtomicStore(path, r, hash)
}

func (s *fileSystemStore) Close() error {
	return nil
}

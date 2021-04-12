package artifact

import (
	"fmt"
	"io"
)

type Store interface {
	Contains(hash string) error
	Load(hash string) (io.ReadCloser, error)
	Store(hash string, r io.Reader) error
}

type StoreType string

const (
	StoreTypeFileSystem    = StoreType("fs")
	StoreTypeGoogleStorage = StoreType("google_storage")
)

func NewStore(config StoreConfig) (Store, error) {
	switch v := config.(type) {
	case *fileSystemStoreConfig:
		return newFileSystemStore(v)
	case *googleStorageStoreConfig:
		return newGoogleStorageStore(v)
	default:
		return nil, fmt.Errorf("unknown store type %q", config.Type())
	}
}

func NewErrArtifactNotFoundHash(hash string) error {
	return &errArtifactNotFound{hash: &hash}
}

func NewErrArtifactNotFoundPath(path string) error {
	return &errArtifactNotFound{path: &path}
}

func IsErrArtifactNotFound(err error) bool {
	_, ok := err.(*errArtifactNotFound)
	return ok
}

type errArtifactNotFound struct {
	path *string
	hash *string
}

func (e *errArtifactNotFound) Error() string {
	if e.path != nil {
		return fmt.Sprintf("artifact not found; path=%q", *e.path)
	}
	return fmt.Sprintf("artifact not found; hash=%q", *e.hash)
}

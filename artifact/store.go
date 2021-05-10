package artifact

import (
	"fmt"
	"io"
)

// A Store is responsible for loading and storing artifacts by their
// hashes and content.
type Store interface {
	Contains(hash string) error
	Load(hash string) (io.ReadCloser, error)
	Store(hash string, r io.Reader) error
}

// A StoreType indentifies a specific type of Store.
type StoreType string

// The set of known store types.
const (
	StoreTypeFileSystem    = StoreType("fs")
	StoreTypeGoogleStorage = StoreType("google_storage")
)

// NewStore returns a new store based on the given config. It errors
// if making the store fails or the underlying type has no constructor.
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

// NewErrArtifactNotFoundHash returns an error for when an artifact
// is not found by its hash.
func NewErrArtifactNotFoundHash(hash string) error {
	return &errArtifactNotFound{hash: &hash}
}

// NewErrArtifactNotFoundPath returns an error for when an artifact
// is not found by its path.
func NewErrArtifactNotFoundPath(path string) error {
	return &errArtifactNotFound{path: &path}
}

// IsErrArtifactNotFound returns if the given error is any kind of
// artifact not found error.
func IsErrArtifactNotFound(err error) bool {
	_, ok := err.(*errArtifactNotFound)
	return ok
}

// An errArtifactNotFound is used when an artifact can not be found.
type errArtifactNotFound struct {
	path *string
	hash *string
}

// Error returns an error specific to the way the artifact was searched for.
func (e *errArtifactNotFound) Error() string {
	if e.path != nil {
		return fmt.Sprintf("artifact not found; path=%q", *e.path)
	}
	return fmt.Sprintf("artifact not found; hash=%q", *e.hash)
}

package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestNewFileSystemStore(t *testing.T) {
	_, err := NewStore(&FileSystemStoreConfig{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such file")

	dir := t.TempDir()
	newDir := filepath.Join(dir, "new")
	_, err = NewStore(&FileSystemStoreConfig{Path: newDir})
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(newDir)
	test.That(t, err, test.ShouldBeNil)

	_, err = NewStore(&FileSystemStoreConfig{Path: "fs_store_test.go"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "directory")

	_, err = NewStore(&FileSystemStoreConfig{Path: dir})
	test.That(t, err, test.ShouldBeNil)
}

func TestFileSystemStore(t *testing.T) {
	dir := t.TempDir()

	store, err := NewStore(&FileSystemStoreConfig{Path: dir})
	test.That(t, err, test.ShouldBeNil)
	testStore(t, store, false)

	store, err = NewStore(&FileSystemStoreConfig{Path: dir})
	test.That(t, err, test.ShouldBeNil)
	testStore(t, store, true)
}

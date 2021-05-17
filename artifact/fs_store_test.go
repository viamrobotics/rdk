package artifact

import (
	"testing"

	"go.viam.com/test"
)

func TestNewFileSystemStore(t *testing.T) {
	_, err := newFileSystemStore(&fileSystemStoreConfig{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such file")

	_, err = newFileSystemStore(&fileSystemStoreConfig{Path: "nothinghere"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such file")

	_, err = newFileSystemStore(&fileSystemStoreConfig{Path: "fs_store_test.go"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "directory")

	dir := t.TempDir()

	_, err = newFileSystemStore(&fileSystemStoreConfig{Path: dir})
	test.That(t, err, test.ShouldBeNil)
}

func TestFileSystemStore(t *testing.T) {
	dir := t.TempDir()

	store, err := newFileSystemStore(&fileSystemStoreConfig{Path: dir})
	test.That(t, err, test.ShouldBeNil)
	testStore(t, store, false)

	store, err = newFileSystemStore(&fileSystemStoreConfig{Path: dir})
	test.That(t, err, test.ShouldBeNil)
	testStore(t, store, true)
}

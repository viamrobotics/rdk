package artifact

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/testutils"
)

func TestEmplaceFile(t *testing.T) {
	storeDir := testutils.TempDir(t, "file_test", "")
	defer os.RemoveAll(storeDir)
	rootDir := testutils.TempDir(t, "file_test", "")
	defer os.RemoveAll(rootDir)

	store, err := newFileSystemStore(&fileSystemStoreConfig{Path: storeDir})
	test.That(t, err, test.ShouldBeNil)

	unknownHash := "foo"
	file1Path := filepath.Join(storeDir, "file1")
	err = emplaceFile(store, unknownHash, file1Path)
	test.That(t, err, test.ShouldResemble, &errArtifactNotFound{hash: &unknownHash})
	_, err = os.Stat(file1Path)
	test.That(t, err, test.ShouldNotBeNil)

	content1 := "mycoolcontent"
	content2 := "myothercoolcontent"

	hashVal1, err := computeHash([]byte(content1))
	test.That(t, err, test.ShouldBeNil)
	hashVal2, err := computeHash([]byte(content2))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, store.Store(hashVal1, strings.NewReader(content1)), test.ShouldBeNil)
	test.That(t, store.Store(hashVal2, strings.NewReader(content2)), test.ShouldBeNil)

	test.That(t, emplaceFile(store, hashVal1, file1Path), test.ShouldBeNil)
	rd, err := ioutil.ReadFile(file1Path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rd), test.ShouldEqual, content1)

	test.That(t, emplaceFile(store, hashVal2, file1Path), test.ShouldBeNil)
	rd, err = ioutil.ReadFile(file1Path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rd), test.ShouldEqual, content2)

	file2Path := filepath.Join(storeDir, "file2")
	test.That(t, emplaceFile(store, hashVal1, file2Path), test.ShouldBeNil)
	rd, err = ioutil.ReadFile(file2Path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rd), test.ShouldEqual, content1)
	rd, err = ioutil.ReadFile(file1Path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rd), test.ShouldEqual, content2)

	file3Path := filepath.Join(storeDir, "one", "two", "three", "file")
	test.That(t, emplaceFile(store, hashVal1, file3Path), test.ShouldBeNil)
	rd, err = ioutil.ReadFile(file3Path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(rd), test.ShouldEqual, content1)
}

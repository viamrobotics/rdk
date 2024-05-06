package shelltestutils

import (
	"testing"

	"go.viam.com/test"
)

func TestDirectoryContentsEqual(t *testing.T) {
	tfs := SetupTestFileSystem(t)

	err := DirectoryContentsEqual(tfs.Root, tfs.SingleFileNested)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not a directory")

	err = DirectoryContentsEqual(tfs.Root, tfs.InnerDir)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "has 8 files")
	test.That(t, err.Error(), test.ShouldContainSubstring, "has 4 files")

	test.That(t, DirectoryContentsEqual(tfs.Root, tfs.Root), test.ShouldBeNil)

	tfs2 := SetupTestFileSystem(t, "diff")
	err = DirectoryContentsEqual(tfs.Root, tfs2.Root)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "contents not equal")
}

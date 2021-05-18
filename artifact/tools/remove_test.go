package tools

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/artifact"
)

func TestRemove(t *testing.T) {
	dir, undo := artifact.TestSetupGlobalCache(t)
	defer undo()
	confPath := filepath.Join(dir, ".artifact.json")
	test.That(t, ioutil.WriteFile(confPath, []byte(`{}`), 0644), test.ShouldBeNil)

	test.That(t, Push(), test.ShouldBeNil)

	filePath := artifact.MustNewPath("some/file")
	test.That(t, os.MkdirAll(filepath.Dir(filePath), 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(filePath, []byte("hello"), 0644), test.ShouldBeNil)
	otherFilePath := artifact.MustNewPath("some/other_file")
	test.That(t, os.MkdirAll(filepath.Dir(otherFilePath), 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(otherFilePath, []byte("world"), 0644), test.ShouldBeNil)

	test.That(t, Push(), test.ShouldBeNil)
	test.That(t, Remove("some/file"), test.ShouldBeNil)
	test.That(t, Remove("some/unknown_file"), test.ShouldBeNil)

	test.That(t, os.RemoveAll(artifact.MustNewPath("/")), test.ShouldBeNil)
	_, err := os.Stat(filePath)
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(otherFilePath)
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, Pull("/", true), test.ShouldBeNil)
	_, err = os.Stat(filePath)
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(otherFilePath)
	test.That(t, err, test.ShouldBeNil)

	// cannot remove root to protect user from accidents. can add it if we get
	// requests for it.
	test.That(t, Remove("/"), test.ShouldBeNil)
	test.That(t, os.RemoveAll(artifact.MustNewPath("/")), test.ShouldBeNil)

	test.That(t, Pull("/", true), test.ShouldBeNil)
	_, err = os.Stat(filePath)
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(otherFilePath)
	test.That(t, err, test.ShouldBeNil)
}

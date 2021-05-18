package tools

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/artifact"
)

func TestClean(t *testing.T) {
	dir, undo := artifact.TestSetupGlobalCache(t)
	defer undo()
	test.That(t, os.MkdirAll(filepath.Join(dir, artifact.DotDir), 0755), test.ShouldBeNil)
	confPath := filepath.Join(dir, artifact.DotDir, artifact.ConfigName)
	test.That(t, ioutil.WriteFile(confPath, []byte(`{}`), 0644), test.ShouldBeNil)

	test.That(t, Clean(), test.ShouldBeNil)

	filePath := artifact.MustNewPath("some/file")
	test.That(t, os.MkdirAll(filepath.Dir(filePath), 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(filePath, []byte("hello"), 0644), test.ShouldBeNil)
	_, err := os.Stat(filePath)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, Clean(), test.ShouldBeNil)
	_, err = os.Stat(filePath)
	test.That(t, err, test.ShouldNotBeNil)
}

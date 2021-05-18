package tools

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/artifact"
)

func TestStatus(t *testing.T) {
	dir, undo := artifact.TestSetupGlobalCache(t)
	defer undo()
	test.That(t, os.MkdirAll(filepath.Join(dir, artifact.DotDir), 0755), test.ShouldBeNil)
	confPath := filepath.Join(dir, artifact.DotDir, artifact.ConfigName)
	test.That(t, ioutil.WriteFile(confPath, []byte(`{}`), 0644), test.ShouldBeNil)

	filePath := artifact.MustNewPath("some/file")
	test.That(t, os.MkdirAll(filepath.Dir(filePath), 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(filePath, []byte("hello"), 0644), test.ShouldBeNil)
	otherFilePath := artifact.MustNewPath("some/other_file")
	test.That(t, os.MkdirAll(filepath.Dir(otherFilePath), 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(otherFilePath, []byte("world"), 0644), test.ShouldBeNil)

	status, err := Status()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, &artifact.Status{
		Unstored: []string{filePath, otherFilePath},
	})

	test.That(t, Push(), test.ShouldBeNil)

	status, err = Status()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, &artifact.Status{})

	test.That(t, ioutil.WriteFile(otherFilePath, []byte("changes"), 0644), test.ShouldBeNil)

	status, err = Status()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, &artifact.Status{
		Modified: []string{otherFilePath},
	})

	newFilePath := artifact.MustNewPath("some/new_file")
	test.That(t, os.MkdirAll(filepath.Dir(newFilePath), 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(newFilePath, []byte("newwwww"), 0644), test.ShouldBeNil)

	status, err = Status()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldResemble, &artifact.Status{
		Unstored: []string{newFilePath},
		Modified: []string{otherFilePath},
	})
}

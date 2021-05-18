package tools

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/artifact"
)

func TestPull(t *testing.T) {
	dir, undo := artifact.TestSetupGlobalCache(t)
	defer undo()

	confPath := filepath.Join(dir, ".artifact.json")
	sourcePath := filepath.Join(dir, "source")
	test.That(t, os.MkdirAll(sourcePath, 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(confPath, []byte(fmt.Sprintf(`{
		"source_store": {
			"type": "fs",
			"path": "%s"
		}
	}`, sourcePath)), 0644), test.ShouldBeNil)
	treePath := filepath.Join(dir, ".artifact.tree.json")
	test.That(t, ioutil.WriteFile(treePath, []byte(`{
		"one": {
			"two": {
				"size": 10,
				"hash": "foo"
			},
			"three": {
				"size": 10,
				"hash": "bar"
			}
		},
		"two": {
			"size": 10,
			"hash": "baz"
		}
	}`), 0644), test.ShouldBeNil)

	store, err := artifact.NewStore(&artifact.FileSystemStoreConfig{Path: sourcePath})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, store.Store("foo", strings.NewReader("foocontent")), test.ShouldBeNil)
	test.That(t, store.Store("bar", strings.NewReader("barcontent")), test.ShouldBeNil)
	test.That(t, store.Store("baz", strings.NewReader("bazcontent")), test.ShouldBeNil)

	test.That(t, Pull("one/two", true), test.ShouldBeNil)

	_, err = os.Stat(artifact.MustNewPath("one/two"))
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(artifact.MustNewPath("one/three"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(artifact.MustNewPath("two"))
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, Pull("/", true), test.ShouldBeNil)

	_, err = os.Stat(artifact.MustNewPath("one/two"))
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(artifact.MustNewPath("one/three"))
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(artifact.MustNewPath("two"))
	test.That(t, err, test.ShouldBeNil)
}

func TestPullLimit(t *testing.T) {
	dir, undo := artifact.TestSetupGlobalCache(t)
	defer undo()

	confPath := filepath.Join(dir, ".artifact.json")
	sourcePath := filepath.Join(dir, "source")
	test.That(t, os.MkdirAll(sourcePath, 0755), test.ShouldBeNil)
	test.That(t, ioutil.WriteFile(confPath, []byte(fmt.Sprintf(`{
		"source_store": {
			"type": "fs",
			"path": "%s"
		},
		"source_pull_size_limit": 3
	}`, sourcePath)), 0644), test.ShouldBeNil)
	treePath := filepath.Join(dir, ".artifact.tree.json")
	test.That(t, ioutil.WriteFile(treePath, []byte(`{
		"one": {
			"two": {
				"size": 10,
				"hash": "foo"
			},
			"three": {
				"size": 10,
				"hash": "bar"
			}
		},
		"two": {
			"size": 10,
			"hash": "baz"
		}
	}`), 0644), test.ShouldBeNil)

	store, err := artifact.NewStore(&artifact.FileSystemStoreConfig{Path: sourcePath})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, store.Store("foo", strings.NewReader("foocontent")), test.ShouldBeNil)
	test.That(t, store.Store("bar", strings.NewReader("barcontent")), test.ShouldBeNil)
	test.That(t, store.Store("baz", strings.NewReader("bazcontent")), test.ShouldBeNil)

	test.That(t, Pull("one/two", false), test.ShouldBeNil)
	_, err = os.Stat(artifact.MustNewPath("one/two"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(artifact.MustNewPath("one/three"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(artifact.MustNewPath("two"))
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, Pull("/", false), test.ShouldBeNil)

	_, err = os.Stat(artifact.MustNewPath("one/two"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(artifact.MustNewPath("one/three"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(artifact.MustNewPath("two"))
	test.That(t, err, test.ShouldNotBeNil)

	test.That(t, Pull("/", true), test.ShouldBeNil)
	_, err = os.Stat(artifact.MustNewPath("one/two"))
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(artifact.MustNewPath("one/three"))
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(artifact.MustNewPath("two"))
	test.That(t, err, test.ShouldBeNil)
}

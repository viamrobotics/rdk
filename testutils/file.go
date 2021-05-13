package testutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

// TempDir creates a temporary directory and fails the test if it cannot.
func TempDir(t *testing.T, dir, pattern string) string {
	t.Helper()

	var err error

	if os.Getenv("USER") == "" {
		dir, err = ioutil.TempDir(dir, pattern)
	} else {
		dir = filepath.Join("/tmp", "core_test", os.Getenv("USER"), dir, pattern)
		err = os.MkdirAll(dir, 0770)
	}
	test.That(t, err, test.ShouldBeNil)
	return dir
}

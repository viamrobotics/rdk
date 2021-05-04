package testutils

import (
	"io/ioutil"
	"testing"

	"go.viam.com/test"
)

// TempDir creates a temporary directory and fails the test if it cannot.
func TempDir(t *testing.T, dir, pattern string) string {
	t.Helper()
	dir, err := ioutil.TempDir(dir, pattern)
	test.That(t, err, test.ShouldBeNil)
	return dir
}

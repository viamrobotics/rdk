package testutils

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

// TempDir creates a temporary directory and fails the test if it cannot.
func TempDir(t *testing.T, dir, pattern string) string {
	t.Helper()

	root := "/tmp"
	if os.Getenv("USER") == "" {
		root = os.TempDir()
	}

	dir = filepath.Join(root, "robotcore_test", os.Getenv("USER"), dir, pattern)
	err := os.MkdirAll(dir, 0770)
	test.That(t, err, test.ShouldBeNil)
	return dir
}

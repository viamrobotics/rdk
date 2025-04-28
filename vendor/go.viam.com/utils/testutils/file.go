package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"go.viam.com/test"
)

// TempDirT creates a temporary directory and fails the test if it cannot.
func TempDirT(tb testing.TB, dir, pattern string) string {
	tb.Helper()
	tempDir, err := TempDir(dir, pattern)
	test.That(tb, err, test.ShouldBeNil)
	return tempDir
}

// TempDir creates a temporary directory and fails the test if it cannot.
func TempDir(dir, pattern string) (string, error) {
	var err error

	if os.Getenv("USER") == "" || filepath.IsAbs(dir) {
		dir, err = os.MkdirTemp(dir, pattern)
	} else {
		dir = filepath.Join("/tmp", fmt.Sprintf("viam-test-%s-%s-%s", os.Getenv("USER"), dir, pattern))
		err = os.MkdirAll(dir, 0o750)
	}
	return dir, err
}

// TempFile returns a unique temporary file named "something.txt" or fails the test if it
// cannot. It automatically closes and removes the file after the test and all its
// subtests complete.
func TempFile(tb testing.TB) *os.File {
	tb.Helper()
	tempFile := filepath.Join(tb.TempDir(), "something.txt")
	//nolint:gosec
	f, err := os.Create(tempFile)
	test.That(tb, err, test.ShouldBeNil)

	tb.Cleanup(func() {
		test.That(tb, f.Close(), test.ShouldBeNil)
		// Since the file was placed in a directory that was created via TB.TempDir, it
		// will automatically be deleted after the test and all its subtests complete, so
		// we do not need to remove it manually.
	})

	return f
}

// WatchedFiles creates a file watcher and n unique temporary files all named
// "something.txt", or fails the test if it cannot. It returns the watcher and a slice of
// files. It automatically closes the watcher, and closes and removes all files after the
// test and all its subtests complete.
//
// For safety, this function will not create more than 50 files.
func WatchedFiles(tb testing.TB, n int) (*fsnotify.Watcher, []*os.File) {
	tb.Helper()

	if n > 50 {
		tb.Fatal("will not create more than 50 temporary files, sorry")
	}

	watcher, err := fsnotify.NewWatcher()
	test.That(tb, err, test.ShouldBeNil)
	tb.Cleanup(func() {
		test.That(tb, watcher.Close(), test.ShouldBeNil)
	})

	var tempFiles []*os.File
	for i := 0; i < n; i++ {
		f := TempFile(tb)
		tempFiles = append(tempFiles, f)
		test.That(tb, watcher.Add(f.Name()), test.ShouldBeNil)
	}

	return watcher, tempFiles
}

// WatchedFile creates a file watcher and a unique temporary file named "something.txt",
// or fails the test if it cannot. It returns the watcher and the file. It automatically
// closes the watcher, and closes and removes the file after the test and all its
// subtests complete.
func WatchedFile(tb testing.TB) (*fsnotify.Watcher, *os.File) {
	tb.Helper()

	watcher, tempFiles := WatchedFiles(tb, 1)
	test.That(tb, len(tempFiles), test.ShouldEqual, 1)

	return watcher, tempFiles[0]
}

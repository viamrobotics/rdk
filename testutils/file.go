package testutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
)

// TempDirT creates a temporary directory and fails the test if it cannot.
func TempDirT(t *testing.T, dir, pattern string) string {
	tempDir, err := TempDir(dir, pattern)
	test.That(t, err, test.ShouldBeNil)
	return tempDir
}

var (
	tempRoot   string
	tempRootMu sync.Mutex
)

// TempDir creates a temporary directory and fails the test if it cannot.
func TempDir(dir, pattern string) (string, error) {
	var err error

	if os.Getenv("USER") == "" || filepath.IsAbs(dir) {
		dir, err = ioutil.TempDir(dir, pattern)
	} else {
		userRoot := filepath.Join("/tmp", "core_test", os.Getenv("USER"))
		if err := os.MkdirAll(userRoot, 0770); err != nil {
			return "", err
		}
		tempRootMu.Lock()
		if tempRoot == "" {
			for {
				ts := fmt.Sprintf("%d", time.Now().UnixNano())
				tempRoot = filepath.Join(userRoot, ts)
				err := os.Mkdir(tempRoot, 0770)
				if err == nil {
					break
				}
				if err != nil && !os.IsExist(err) {
					return "", err
				}
				time.Sleep(time.Second)
			}
		}
		tempRootMu.Unlock()
		dir = filepath.Join(tempRoot, dir, pattern)
		err = os.MkdirAll(dir, 0770)
	}
	return dir, err
}

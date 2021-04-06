package testutils

import (
	"os"
	"path/filepath"
)

func LargeFileTestPath(path string) string {
	return filepath.Join(os.Getenv("HOME"), "/Dropbox/echolabs_data/", path)
}

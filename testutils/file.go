package testutils

import (
	"os"
	"path/filepath"
	"runtime"
)

func ResolveFile(fn string) string {
	_, thisFilePath, _, _ := runtime.Caller(0)
	thisDirPath, err := filepath.Abs(filepath.Dir(thisFilePath))
	if err != nil {
		panic(err)
	}
	return filepath.Join(thisDirPath, "..", fn)
}

func LargeFileTestPath(path string) string {
	return filepath.Join(os.Getenv("HOME"), "/Dropbox/echolabs_data/", path)
}

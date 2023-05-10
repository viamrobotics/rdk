package utils

import (
	"os"
	"path/filepath"
	"runtime"

	"go.viam.com/utils"
)

// ResolveFile returns the path of the given file relative to the root
// of the codebase. For example, if this file currently
// lives in utils/file.go and ./foo/bar/baz is given, then the result
// is foo/bar/baz. This is helpful when you don't want to relatively
// refer to files when you're not sure where the caller actually
// lives in relation to the target file.
func ResolveFile(fn string) string {
	//nolint:dogsled
	_, thisFilePath, _, _ := runtime.Caller(0)
	thisDirPath, err := filepath.Abs(filepath.Dir(thisFilePath))
	if err != nil {
		panic(err)
	}
	return filepath.Join(thisDirPath, "..", fn)
}

// RemoveFileNoError will remove the file at the given path if it exists. Any
// errors will be suppressed.
func RemoveFileNoError(path string) {
	utils.UncheckedErrorFunc(func() error {
		if _, err := os.Stat(path); err == nil {
			return os.Remove(path)
		}
		return nil
	})
}

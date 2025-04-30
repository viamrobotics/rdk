package utils

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
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

// SafeJoinDir performs a filepath.Join of 'parent' and 'subdir' but returns an error
// if the resulting path points outside of 'parent'.
// See also https://github.com/cyphar/filepath-securejoin.
func SafeJoinDir(parent, subdir string) (string, error) {
	res := filepath.Join(parent, subdir)
	if !strings.HasPrefix(filepath.Clean(res), filepath.Clean(parent)+string(os.PathSeparator)) {
		return res, errors.Errorf("unsafe path join: '%s' with '%s'", parent, subdir)
	}
	return res, nil
}

// ExpandHomeDir expands "~/x/y" to use homedir.
func ExpandHomeDir(path string) (string, error) {
	// note: do not simplify this logic unless you are testing cross platform.
	// Windows supports both kinds of slash, we don't want to only test for "\\" on win.
	if path == "~" ||
		strings.HasPrefix(path, "~/") ||
		(runtime.GOOS == "windows" && strings.HasPrefix(path, "~\\")) {
		usr, err := user.Current()
		if err != nil {
			return "", errors.Wrap(err, "expanding home dir")
		}
		return filepath.Join(usr.HomeDir, path[min(2, len(path)):]), nil
	}
	return path, nil
}

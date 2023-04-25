package utils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
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

// BuildInDir will run "go build ." in the provided RDK directory and return
// any build related errors.
func BuildInDir(dir string) error {
	builder := exec.Command("go", "build", ".")
	builder.Dir = ResolveFile(dir)
	out, err := builder.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build error: %w", err)
	}
	if string(out) != "" {
		return fmt.Errorf(`unexpected output from "go build .": %v`, out)
	}
	return nil
}

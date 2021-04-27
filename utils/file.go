package utils

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

func ResolveSharedDir(argDir string) string {
	calledBinary, err := os.Executable()
	if err != nil {
		panic(err)
	}

	if argDir != "" {
		return argDir
	} else if calledBinary == "/usr/bin/viam-server" {
		if _, err := os.Stat("/usr/share/viam"); !os.IsNotExist(err) {
			return "/usr/share/viam"
		}
	}
	return ResolveFile("robot/web/runtime-shared")
}

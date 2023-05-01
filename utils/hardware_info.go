package utils

import (
	"runtime"
)

func getArchitectureInfo() string {
	var arch string = runtime.GOARCH
	return arch
}

func getOSInfo() string {
	var os string = runtime.GOOS
	return os
}

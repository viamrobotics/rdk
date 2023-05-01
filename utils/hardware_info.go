package utils

import (
	"runtime"
)

func GetArchitectureInfo() string {
	var arch string = runtime.GOARCH
	return arch
}

func GetOSInfo() string {
	var os string = runtime.GOOS
	return os
}

package utils

import (
	"runtime"
)

// returns the architecture of the board
func GetArchitectureInfo() string {
	var arch string = runtime.GOARCH
	return arch
}

// returns the OS of the board.
func GetOSInfo() string {
	var os string = runtime.GOOS
	return os
}

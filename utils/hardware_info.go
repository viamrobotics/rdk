package utils

import (
	"runtime"
)

// GetArchitectureInfo returns the architecture of the board.
func GetArchitectureInfo() string {
	return runtime.GOARCH
}

// GetOSInfo returns the OS of the board.
func GetOSInfo() string {
	return runtime.GOOS
}

//go:build !windows

package utils

import (
	"os"
	"slices"
)

// ViamTCPSockets returns true if an env is set or if the platform requires it.
func ViamTCPSockets() bool {
	return slices.Contains(EnvTrueValues, os.Getenv("VIAM_TCP_SOCKETS"))
}

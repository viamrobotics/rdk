//go:build !windows

package utils

import (
	"fmt"
	"os"
	"slices"
)

// OnlyUseViamTCPSockets returns true if TCP sockets should be used in lieu of Unix sockets.
func OnlyUseViamTCPSockets() (use bool, reason string) {
	envVal := os.Getenv(ViamTCPSocketsEnvVar)
	if slices.Contains(EnvTrueValues, envVal) {
		return true, fmt.Sprintf("Env var %s=%s", ViamTCPSocketsEnvVar, envVal)
	}
	return false, ""
}

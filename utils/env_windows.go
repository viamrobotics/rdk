package utils

import (
	"fmt"
	"os"
	"slices"

	"golang.org/x/sys/windows"
)

// OnlyUseViamTCPSockets returns true if TCP sockets should be used in lieu of Unix sockets.
func OnlyUseViamTCPSockets() (use bool, reason string) {
	defaultUseTCPSockets := false

	// Windows builds older than 17063 do not support Unix sockets.
	// 2017 support announcement: https://devblogs.microsoft.com/commandline/af_unix-comes-to-windows/
	// TODO: investigate if RtlGetVersion should be called in init and cached.
	if v := windows.RtlGetVersion(); v.BuildNumber != 0 && v.BuildNumber < 17063 {
		defaultUseTCPSockets = true
		reason = fmt.Sprintf("Detected Windows build number <17063: %+v", v)
		// could also return early here, but user may appreciate reason string with env var value.
	}
	envVal := os.Getenv(ViamTCPSocketsEnvVar)
	if envVal == "" {
		return defaultUseTCPSockets, reason
	} else if slices.Contains(EnvTrueValues, envVal) {
		return true, fmt.Sprintf("Env var %s=%s", ViamTCPSocketsEnvVar, envVal)
	}
	// note: the control flow here means that any non-empty, non-truthy value is false.
	// this allows you to override the detected default if needed.
	return false, ""
}

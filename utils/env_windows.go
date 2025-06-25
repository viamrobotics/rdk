package utils

import (
	"os"
	"slices"

	"golang.org/x/sys/windows"
)

// ViamTCPSockets returns true if an env is set or if the platform requires it.
func ViamTCPSockets() bool {
	// 2017 support announcement: https://devblogs.microsoft.com/commandline/af_unix-comes-to-windows/
	defaultVal := false
	if v := windows.RtlGetVersion(); v.BuildNumber != 0 && v.BuildNumber < 17063 {
		defaultVal = true
	}
	envVal := os.Getenv("VIAM_TCP_SOCKETS")
	if envVal == "" {
		return defaultVal
	}
	// note: the control flow here means that any non-empty, non-truthy value is false.
	return slices.Contains(EnvTrueValues, envVal)
}

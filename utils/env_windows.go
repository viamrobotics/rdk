package utils

import (
	"fmt"
	"os"
	"slices"

	"golang.org/x/sys/windows"
)

// ViamTCPSockets returns true if an env is set or if the platform requires it.
func ViamTCPSockets() bool {
	// 2017 support announcement: https://devblogs.microsoft.com/commandline/af_unix-comes-to-windows/
	defaultVal := false

	v := windows.RtlGetVersion()
	if v.BuildNumber != 0 && v.BuildNumber < 17063 {
		defaultVal = true
	}
	envVal := os.Getenv("VIAM_TCP_SOCKETS")
	fmt.Printf("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~RtlGetVersion %+v envVal=%v\n", v, envVal)
	if envVal == "" {
		return defaultVal
	}
	// note: the control flow here means that any non-empty, non-truthy value is false.
	return slices.Contains(EnvTrueValues, envVal)
}

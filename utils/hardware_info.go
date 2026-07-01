package utils

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"

	"braces.dev/errtrace"
	"go.viam.com/utils"
)

// GetDeviceInfo returns the device information in stringset.
func GetDeviceInfo(modelName string) (utils.StringSet, error) {
	arch := runtime.GOARCH

	switch {
	case strings.HasPrefix(arch, "amd"):
		return errtrace.Wrap2(stringSetFromX86(modelName))
	case strings.HasPrefix(arch, "arm"):
		return errtrace.Wrap2(stringSetFromARM(modelName))
	default:
		return nil, errtrace.Wrap(noBoardError(modelName))
	}
}

// A helper function for ARM architecture to process contents of the
// device path and returns the compatible device information.
func stringSetFromARM(modelName string) (utils.StringSet, error) {
	const path = "/proc/device-tree/compatible"
	compatiblesRd, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errtrace.Wrap(noBoardError(modelName))
		}
		return nil, errtrace.Wrap(err)
	}

	compatiblesStr := string(compatiblesRd)
	// Remove any initial or final null bytes, then split on the rest of them.
	compatiblesStr = strings.Trim(compatiblesStr, "\x00")
	return utils.NewStringSet(strings.Split(compatiblesStr, "\x00")...), nil
}

// A helper function for AMD architecture to process contents of the
// device path and returns the compatible device information.
func stringSetFromX86(modelName string) (utils.StringSet, error) {
	const path = "/sys/devices/virtual/dmi/id/board_name"
	compatiblesRd, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errtrace.Wrap(noBoardError(modelName))
		}
		return nil, errtrace.Wrap(err)
	}

	compatiblesRd = bytes.TrimSpace(compatiblesRd)

	return utils.NewStringSet(string(compatiblesRd)), nil
}

func noBoardError(modelName string) error {
	return errtrace.Wrap(fmt.Errorf("could not determine %q model", modelName))
}

package utils

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.viam.com/utils"
)

// GetArchitectureInfo returns the architecture of the board.
func GetArchitectureInfo(modelName string) (utils.StringSet, error) {
	arch := runtime.GOARCH

	switch {
	case strings.HasPrefix(arch, "amd"):
		return stringSetFromX86(modelName)
	case strings.HasPrefix(arch, "arm"):
		return stringSetFromARM(modelName)
	default:
		return nil, noBoardError(modelName)
	}
}

// A helper function for ARM architecture to process contents of a
// given content of a file from os.ReadFile.
func stringSetFromARM(modelName string) (utils.StringSet, error) {
	path := "/proc/device-tree/compatible"
	compatiblesRd, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, noBoardError(modelName)
		}
		return nil, err
	}

	return utils.NewStringSet(strings.Split(string(compatiblesRd), "\x00")...), nil
}

func stringSetFromX86(modelName string) (utils.StringSet, error) {
	path := "/sys/devices/virtual/dmi/id/board_name"
	compatiblesRd, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, noBoardError(modelName)
		}
		return nil, err
	}

	// Remove whitespace and null characters from the content
	compatiblesRd = bytes.TrimSpace(compatiblesRd)
	compatiblesRd = bytes.ReplaceAll(compatiblesRd, []byte{0x00}, []byte{})

	return utils.NewStringSet(string(compatiblesRd)), nil
}

func noBoardError(modelName string) error {
	return fmt.Errorf("could not determine %q model", modelName)
}

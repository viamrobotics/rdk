package utils

// #include <sys/utsname.h>
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"go.viam.com/utils"
)

// GetDeviceInfo returns the device information in stringset.
func GetDeviceInfo(modelName string) (utils.StringSet, error) {
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

// A helper function for ARM architecture to process contents of the
// device path and returns the compatible device information.
func stringSetFromARM(modelName string) (utils.StringSet, error) {
	const path = "/proc/device-tree/compatible"
	compatiblesRd, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, noBoardError(modelName)
		}
		return nil, err
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
			return nil, noBoardError(modelName)
		}
		return nil, err
	}

	compatiblesRd = bytes.TrimSpace(compatiblesRd)

	return utils.NewStringSet(string(compatiblesRd)), nil
}

func noBoardError(modelName string) error {
	return fmt.Errorf("could not determine %q model", modelName)
}

// OSInformation contains information about the OS
// that the camera is running on.
type OSInformation struct {
	Name   string // e.g. "linux"
	Arch   string // e.g. "arm64"
	Kernel string // e.g. "4.9.140-tegra"
	Device string // e.g. "NVIDIA Jetson AGX Xavier"
}

// DetectOSInformation pulls relevant OS attributes as an OSInformation struct
// Kernel and Device will be "unknown" if unable to retrieve info from the filesystem
// returns an error if kernel version or device name is unavailable
func DetectOSInformation() (OSInformation, error) {
	kernelVersion, err := getKernelVersion()
	if err != nil {
		return OSInformation{}, fmt.Errorf("failed to get kernel version: %w", err)
	}
	deviceName, err := getDeviceName()
	if err != nil {
		return OSInformation{}, fmt.Errorf("failed to get device name: %w", err)
	}
	osInfo := OSInformation{
		Name:   runtime.GOOS,
		Arch:   runtime.GOARCH,
		Kernel: kernelVersion,
		Device: deviceName,
	}
	return osInfo, nil
}

// getKernelVersion returns the Linux kernel version
// $ uname -r
func getKernelVersion() (string, error) {
	var utsName C.struct_utsname
	if C.uname(&utsName) == -1 {
		return "", fmt.Errorf("uname information unavailable (%v)", utsName)
	}
	release := C.GoString((*C.char)(unsafe.Pointer(&utsName.release[0])))
	return release, nil
}

// getDeviceName returns the model name of the device
// $ cat /sys/firmware/devicetree/base/model
func getDeviceName() (string, error) {
	const devicePath = "/sys/firmware/devicetree/base/model"
	device, err := os.ReadFile(devicePath)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(device, "\x00")), nil
}

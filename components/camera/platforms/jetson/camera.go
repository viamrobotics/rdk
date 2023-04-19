package jetsoncamera

// #include <sys/utsname.h>
// #include <string.h>
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

// GetOSInformation pulls relevant OS attributes as an OSInformation struct
// Kernel and Device will be "unkown" if unable to retrieve info from the filesystem
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

// getKernelVersion returns the Linux kernel version ($ uname -r)
func getKernelVersion() (string, error) {
	var utsName C.struct_utsname
	if C.uname(&utsName) == -1 {
		return Unknown, fmt.Errorf("uname information unavailable (%v)", utsName)
	}
	release := C.GoString((*C.char)(unsafe.Pointer(&utsName.release[0])))
	return release, nil
}

// getDeviceName returns the model name of the device
// ($ cat /sys/firmware/devicetree/base/model)
func getDeviceName() (string, error) {
	devicePath := "/sys/firmware/devicetree/base/model"
	device, err := os.ReadFile(devicePath)
	if err != nil {
		return Unknown, err
	}
	return string(bytes.TrimRight(device, "\x00")), nil
}

// Validate checks if the daughterboard and driver are supported and installed on the device
func Validate(osInfo OSInformation, daughterboardName string, driverName string) error {
	board, ok := cameraInfoMappings[osInfo.Device]
	if !ok {
		return fmt.Errorf("the %s device is not supported on this platform", osInfo.Device)
	}
	daughterboard, ok := board.Daughterboards[daughterboardName]
	if !ok {
		return fmt.Errorf("the %s daughterboard is not supported on this platform", daughterboardName)
	}
	driver, ok := board.Modules[driverName]
	if !ok {
		return fmt.Errorf("the %s driver is not supported on this platform", driverName)
	}
	if err := checkDaughterBoardConnected(daughterboard); err != nil {
		return fmt.Errorf("the %s daughterboard is not connected or not powerd on. Please check daughter-board conenction to the %s", daughterboardName, osInfo.Device)
	}
	if err := checkDriverInstalled(osInfo.Kernel, driver); err != nil {
		return fmt.Errorf("the %s driver not installed. Please follow instructions for driver installation", driverName)
	}

	return fmt.Errorf("the %s daughterboard is connected and %s camera driver is installed on the %s. please check that the video path is correct and driver is working", daughterboardName, driverName, osInfo.Device)
}

// checkDaughterBoardConnected checks if the daughterboard is connected
// by looking for the I2C bus interfaces associated with the daughterboard
func checkDaughterBoardConnected(daughterboard []string) error {
	for _, i2c := range daughterboard {
		err := checkI2CInterface(i2c)
		if err != nil {
			return fmt.Errorf("unable to verify that daughterboard is connected: %w", err)
		}
	}
	return nil
}

// checkI2CInterface checks if the I2C bus is available
func checkI2CInterface(bus string) error {
	i2cPath := filepath.Join("/dev", bus)
	if err := checkFileExists(i2cPath); err != nil {
		return fmt.Errorf("unable to verify that i2c bus is available: %w", err)
	}
	return nil
}

// checkDriverInstalled checks if the driver is installed for the
// given kernel version and object file target
func checkDriverInstalled(kernel string, driver string) error {
	driverPath := filepath.Join("/lib/modules", kernel, "extra", driver)
	if err := checkFileExists(driverPath); err != nil {
		return fmt.Errorf("unable to verify that camera driver is installed: %w", err)
	}
	return nil
}

// checkFileExists is a helper function that wraps os.Stat
// errors are parsed to return a more specific error message
func checkFileExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", path)
	} else if err != nil {
		switch {
		case os.IsPermission(err):
			return fmt.Errorf("permission denied when trying to access %s", path)
		case errors.Is(err, syscall.ENOSPC):
			return fmt.Errorf("device is out of space. unable to check %s", path)
		case errors.Is(err, syscall.ENOMEM):
			return fmt.Errorf("device is out of memory. unable to check %s", path)
		default:
			return fmt.Errorf("unable to access %s", path)
		}
	} else {
		return nil
	}
}

package jetsoncamera

// #include <sys/utsname.h>
// #include <string.h>
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"go.uber.org/multierr"
)

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
		return Unknown, fmt.Errorf("uname information unavailable (%v)", utsName)
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
		return Unknown, err
	}
	return string(bytes.TrimRight(device, "\x00")), nil
}

// ValidateSetup wraps an error from NewWebcamSource with a more helpful message
func ValidateSetup(deviceName, daughterboardName, driverName string, err error) error {
	osInfo, osErr := DetectOSInformation()
	if osErr != nil {
		return err
	}
	if osInfo.Device != deviceName {
		return err
	}
	detectErr := DetectError(osInfo, daughterboardName, driverName)
	if detectErr != nil {
		return multierr.Append(err, detectErr)
	}
	return err
}

// DetectError checks daughterboard and camera setup to determine
// our best guess of what is wrong with an unsuccessful camera open
func DetectError(osInfo OSInformation, daughterboardName, driverName string) error {
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
		return fmt.Errorf("the %s daughterboard is not connected or not powerd on."+
			"Please check daughter-board connection to the %s",
			daughterboardName, osInfo.Device)
	}
	if err := checkDriverInstalled(osInfo.Kernel, driver); err != nil {
		return fmt.Errorf("the %s driver not installed. Please follow instructions for driver installation", driverName)
	}

	return fmt.Errorf("the %s daughterboard is connected and "+
		"%s camera driver is installed on the %s."+
		"please check that the video path is correct and driver is working",
		daughterboardName, driverName, osInfo.Device)
}

// checkDaughterBoardConnected checks if the daughterboard is connected
// by looking for the I2C bus interfaces associated with the board
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
func checkDriverInstalled(kernel, driver string) error {
	driverPath := filepath.Join("/lib/modules", kernel, "extra", driver)
	if err := checkFileExists(driverPath); err != nil {
		return fmt.Errorf("unable to verify that camera driver is installed: %w", err)
	}
	return nil
}

// checkFileExists is a helper function that wraps os.Stat
func checkFileExists(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("unable to access %s: %w", path, err)
	}
	return nil
}

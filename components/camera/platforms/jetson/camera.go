package jetsoncamera

// #include <sys/utsname.h>
// #include <string.h>
import "C"

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"unsafe"
)

// GetOSInformation pulls relevant OS attributes as an OSInformation struct
// Kernel and Device will be "unkown" if unable to retrieve info from the filesystem
// returns an error if unable to retrieve kernel version or device name
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
		return Unknown, fmt.Errorf("uname information unavailable")
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
	err := checkDaughterBoardConnected(daughterboard)
	if err != nil {
		return fmt.Errorf("the %s daughterboard is not connected or not powerd on. Please check daughter-board conenction to the %s", daughterboardName, osInfo.Device)
	}
	err = checkDriverInstalled(osInfo.Kernel, driver)
	if err != nil {
		return fmt.Errorf("the %s driver not installed. Please follow instructions for driver installation", driverName)
	}

	return fmt.Errorf("the %s daughterboard is connected and %s camera driver is installed on the %s. please check that the video path is correct and driver is working", daughterboardName, driverName, osInfo.Device)
}

// checkDaughterBoardConnected checks if the daughterboard is connected
// by looking for the I2C bus interfaces
func checkDaughterBoardConnected(daughterboard []string) error {
	// iterate through the daughterboard list
	for _, i2c := range daughterboard {
		// check if the i2c interface is available
		err := checkI2CInterface(i2c)
		if err != nil {
			return fmt.Errorf("the e-CAM20_CUOAGX daughter-board is not connected or not powerd on. please check daughter-board conenction to the Orin AGX over the J509 connector")
		}
	}
	return nil
}

// checkI2CInterface checks if the I2C bus is available
func checkI2CInterface(i2c string) error {
	i2cPath := "/dev/" + i2c
	if _, err := os.Stat(i2cPath); os.IsNotExist(err) {
		return fmt.Errorf("i2c interface %s not available", i2c)
	} else {
		return nil
	}
}

// checkDriverInstalled checks if the driver is installed for the
// given kernel version and object file target
func checkDriverInstalled(kernel string, driver string) error {
	driverPath := "/lib/modules/" + kernel + "/extra/" + driver
	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		return fmt.Errorf("driver %s not installed", driver)
	} else {
		return nil
	}
}

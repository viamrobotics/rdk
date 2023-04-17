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
// Kernel and Device will return "unkown" if unable to retrieve info from the filesystem
func DetectOSInformation() OSInformation {
	osInfo := OSInformation{
		Name:   runtime.GOOS,
		Arch:   runtime.GOARCH,
		Kernel: getKernelVersion(),
		Device: getDeviceName(),
	}
	return osInfo
}

// getKernelVersion returns the kernel version
func getKernelVersion() string {
	var utsName C.struct_utsname
	if C.uname(&utsName) == -1 {
		return Unknown
	}
	release := C.GoString((*C.char)(unsafe.Pointer(&utsName.release[0])))
	return release
}

// getDeviceName returns the model name of the board
func getDeviceName() string {
	devicePath := "/sys/firmware/devicetree/base/model"
	device, err := os.ReadFile(devicePath)
	if err != nil {
		return Unknown
	}
	return string(bytes.TrimRight(device, "\x00"))
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

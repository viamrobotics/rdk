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
// Kenrnel and Device will be "unkown" if unable to retreive attribute from the OS
func DetectOSInformation() OSInformation {
	osInfo := OSInformation{
		Name:   runtime.GOOS,       // e.g. "linux"
		Arch:   runtime.GOARCH,     // e.g. "arm64"
		Kernel: getKernelVersion(), // e.g. "4.9.140-tegra"
		Device: getDeviceName(),    // e.g. "NVIDIA Jetson AGX Xavier"
	}
	return osInfo
}

// getKernelVersion returns the kernel version
// or "unkown" if unable to retreive
func getKernelVersion() string {
	var utsName C.struct_utsname
	if C.uname(&utsName) == -1 {
		return "unknown"
	}
	release := C.GoString((*C.char)(unsafe.Pointer(&utsName.release[0])))
	return release
}

// getDeviceName returns the model name of the board
// or "unkown" if unable to retreive
func getDeviceName() string {
	devicePath := "/sys/firmware/devicetree/base/model"
	device, err := os.ReadFile(devicePath)
	if err != nil {
		return "unknown"
	}
	return string(bytes.TrimRight(device, "\x00"))
}

func PrintError(osInfo OSInformation, driver string) string {
	value, ok := cameraInfoMappings[driver]
	if ok {
		err := checkDriverInstalled(osInfo.Kernel, value.Module)
		if err != nil {
			return "The E-Con Systems " + driver + " driver  not installed. Please follow instructions for driver installation and verify with 'dmesg | grep ar0234' command."
		}
		for _, i2c := range value.I2C {
			err := checkI2CInterface(i2c)
			if err != nil {
				return "The e-CAM20_CUOAGX daughter-board is not connected or not powerd on. Please check daughter-board conenction to the Orin AGX over the J509 connector."
			}
		}
		return "The e-CAM20_CUOAGX daughter-board is connected and the " + driver + " driver is installed, but the video capture interface requested is not avialable. Please ensure camera is connected, driver is working correctly, and the video interface is available."
	} else {
		return "The " + driver + " driver is not supported on this platform."
	}
}

// checkI2CInterface checks if the i2c interface is available
func checkI2CInterface(i2c string) error {
	i2cPath := "/dev/" + i2c
	if _, err := os.Stat(i2cPath); os.IsNotExist(err) {
		return fmt.Errorf("i2c interface %s not available", i2c)
	} else {
		return nil
	}
}

// checkDriverInstalled checks if the driver is installed
// for the given kernel version and object file target
func checkDriverInstalled(kernel string, driver string) error {
	driverPath := "/lib/modules/" + kernel + "/extra/" + driver
	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		return fmt.Errorf("driver %s not installed", driver)
	} else {
		return nil
	}
}

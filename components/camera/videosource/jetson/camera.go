package jetsoncamera

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"syscall"
)

func GetOSInformation() OSInformation {
	osInfo := OSInformation{
		Name:   runtime.GOOS,
		Arch:   runtime.GOARCH,
		Kernel: getKernelVersion(),
		Device: getDeviceName(),
	}
	return osInfo
}

func getKernelVersion() string {
	var utsname syscall.Utsname
	if err := syscall.Uname(&utsname); err != nil {
		return ""
	}
	var release []byte
	for _, b := range utsname.Release {
		if b == 0 {
			break
		}
		release = append(release, byte(b))
	}
	return string(release)
}

func getDeviceName() string {
	devicePath := "/sys/firmware/devicetree/base/model"
	device, err := os.ReadFile(devicePath)
	if err != nil {
		return ""
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

func checkI2CInterface(i2c string) error {
	i2cPath := "/dev/" + i2c
	if _, err := os.Stat(i2cPath); os.IsNotExist(err) {
		return fmt.Errorf("i2c interface %s not available", i2c)
	} else {
		return nil
	}
}

func checkDriverInstalled(kernel string, driver string) error {
	driverPath := "/lib/modules/" + kernel + "/extra/" + driver
	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		return fmt.Errorf("driver %s not installed", driver)
	} else {
		return nil
	}
}

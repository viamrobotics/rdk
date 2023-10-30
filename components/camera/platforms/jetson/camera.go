package jetsoncamera

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/multierr"

	"go.viam.com/rdk/utils"
)

// ValidateSetup wraps an error from NewWebcamSource with a more helpful message.
func ValidateSetup(deviceName, daughterboardName, driverName string, err error) error {
	osInfo, osErr := utils.DetectOSInformation()
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
// our best guess of what is wrong with an unsuccessful camera open.
func DetectError(osInfo utils.OSInformation, daughterboardName, driverName string) error {
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
// by looking for the I2C bus interfaces associated with the board.
func checkDaughterBoardConnected(daughterboard []string) error {
	for _, i2c := range daughterboard {
		err := checkI2CInterface(i2c)
		if err != nil {
			return fmt.Errorf("unable to verify that daughterboard is connected: %w", err)
		}
	}
	return nil
}

// checkI2CInterface checks if the I2C bus is available.
func checkI2CInterface(bus string) error {
	i2cPath := filepath.Join("/dev", bus)
	if err := checkFileExists(i2cPath); err != nil {
		return fmt.Errorf("unable to verify that i2c bus is available: %w", err)
	}
	return nil
}

// checkDriverInstalled checks if the driver is installed for the
// given kernel version and object file target.
func checkDriverInstalled(kernel, driver string) error {
	driverPath := filepath.Join("/lib/modules", kernel, "extra", driver)
	if err := checkFileExists(driverPath); err != nil {
		return fmt.Errorf("unable to verify that camera driver is installed: %w", err)
	}
	return nil
}

// checkFileExists is a helper function that wraps os.Stat.
func checkFileExists(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("unable to access %s: %w", path, err)
	}
	return nil
}

// Package gpsutils contains GPS-related code shared between multiple components. This file
// describes data structures used to configure reading from NMEA devices.
package gpsutils

import (
	"go.viam.com/rdk/resource"
)

// SerialConfig is used for converting Serial NMEA MovementSensor config attributes.
type SerialConfig struct {
	SerialPath     string `json:"serial_path"`
	SerialBaudRate int    `json:"serial_baud_rate,omitempty"`

	// TestChan is a fake "serial" path for test use only
	TestChan chan []uint8 `json:"-"`
}

// I2CConfig is used for converting Serial NMEA MovementSensor config attributes.
type I2CConfig struct {
	I2CBus      string `json:"i2c_bus"`
	I2CAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *I2CConfig) Validate(path string) error {
	if cfg.I2CBus == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2CAddr == 0 {
		return resource.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}
	return nil
}

// Validate ensures all parts of the config are valid.
func (cfg *SerialConfig) Validate(path string) error {
	if cfg.SerialPath == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

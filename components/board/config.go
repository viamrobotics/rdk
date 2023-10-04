package board

import (
	"go.viam.com/utils"
)

// SPIConfig enumerates a specific, shareable SPI bus.
type SPIConfig struct {
	Name      string `json:"name"`
	BusSelect string `json:"bus_select"` // "0" or "1" for main/aux in libpigpio
}

// Validate ensures all parts of the config are valid.
func (config *SPIConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

// I2CConfig enumerates a specific, shareable I2C bus.
type I2CConfig struct {
	Name string `json:"name"`
	Bus  string `json:"bus"`
}

// Validate ensures all parts of the config are valid.
func (config *I2CConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.Bus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "bus")
	}
	return nil
}

// AnalogConfig describes the configuration of an analog reader on a board.
type AnalogConfig struct {
	Name              string `json:"name"`
	Pin               string `json:"pin"`         // analog input pin on the ADC itself
	SPIBus            string `json:"spi_bus"`     // name of the SPI bus (which is configured elsewhere in the config file)
	ChipSelect        string `json:"chip_select"` // the CS line for the ADC chip, typically a pin number on the board
	AverageOverMillis int    `json:"average_over_ms,omitempty"`
	SamplesPerSecond  int    `json:"samples_per_sec,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *AnalogConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

// DigitalInterruptConfig describes the configuration of digital interrupt for a board.
type DigitalInterruptConfig struct {
	Name    string `json:"name"`
	Pin     string `json:"pin"`
	Type    string `json:"type,omitempty"` // e.g. basic, servo
	Formula string `json:"formula,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *DigitalInterruptConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.Pin == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "pin")
	}
	return nil
}

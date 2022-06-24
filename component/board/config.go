package board

import (
	"fmt"

	"go.viam.com/utils"

	"go.viam.com/rdk/config"
)

// RegisterConfigAttributeConverter registers a board.Config converter.
func RegisterConfigAttributeConverter(model string) {
	config.RegisterComponentAttributeMapConverter(
		SubtypeName,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	I2Cs              []I2CConfig              `json:"i2cs,omitempty"`
	SPIs              []SPIConfig              `json:"spis,omitempty"`
	Analogs           []AnalogConfig           `json:"analogs,omitempty"`
	DigitalInterrupts []DigitalInterruptConfig `json:"digital_interrupts,omitempty"`
	Attributes        config.AttributeMap      `json:"attributes,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	for idx, conf := range config.SPIs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "spis", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.I2Cs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "i2cs", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.Analogs {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return err
		}
	}
	for idx, conf := range config.DigitalInterrupts {
		if err := conf.Validate(fmt.Sprintf("%s.%s.%d", path, "digital_interrupts", idx)); err != nil {
			return err
		}
	}
	return nil
}

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
	AverageOverMillis int    `json:"average_over_ms"`
	SamplesPerSecond  int    `json:"samples_per_sec"`
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
	Type    string `json:"type"` // e.g. basic, servo
	Formula string `json:"formula"`
}

// Validate ensures all parts of the config are valid.
func (config *DigitalInterruptConfig) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	return nil
}

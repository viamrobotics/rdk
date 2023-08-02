package genericlinux

import (
	"fmt"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	I2Cs              []board.I2CConfig              `json:"i2cs,omitempty"`
	SPIs              []board.SPIConfig              `json:"spis,omitempty"`
	Analogs           []board.AnalogConfig           `json:"analogs,omitempty"`
	DigitalInterrupts []board.DigitalInterruptConfig `json:"digital_interrupts,omitempty"`
	Attributes        utils.AttributeMap             `json:"attributes,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	for idx, c := range conf.SPIs {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "spis", idx)); err != nil {
			return nil, err
		}
	}
	for idx, c := range conf.I2Cs {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "i2cs", idx)); err != nil {
			return nil, err
		}
	}
	for idx, c := range conf.Analogs {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return nil, err
		}
	}
	for idx, c := range conf.DigitalInterrupts {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "digital_interrupts", idx)); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// UnderlyingConfig is a struct containing absolutely everything a genericlinux board might need
// configured. It is a union of the configs for the customlinux boards and the genericlinux boards
// with static pin definitions, because those components all use the same underlying code but have
// different config types (e.g., only genericlinux has named I2C and SPI buses, while only
// customlinux can change its pin definitions during reconfiguration). The UnderlyingConfig struct
// is a unification of the two of them. Whenever we go through reconfiguration, we convert the
// provided config into this type, and then reconfigure based on this.
type UnderlyingConfig struct {
	I2Cs              []board.I2CConfig
	SPIs              []board.SPIConfig
	Analogs           []board.AnalogConfig
	DigitalInterrupts []board.DigitalInterruptConfig
	GpioMappings      map[string]GPIOBoardMapping
}

// We'll use one of these to turn whatever config we get during reconfiguration into an
// UnderlyingConfig, and then reconfigure based on that.
type ConfigConverter = func (resource.Config) (UnderlyingConfig, error)

func ConstPinDefs(gpioMappings map[string]GPIOBoardMapping) ConfigConverter {
	return func (conf resource.Config) (UnderlyingConfig, error) {
		newConf, err := resource.NativeConfig[*Config](conf)
		if err != nil {
			return UnderlyingConfig{}, err
		}

		return UnderlyingConfig{
			I2Cs:              newConf.I2Cs,
			SPIs:              newConf.SPIs,
			Analogs:           newConf.Analogs,
			DigitalInterrupts: newConf.DigitalInterrupts,
			GpioMappings:      gpioMappings,
		}, nil
	}
}

package genericlinux

import (
	"fmt"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	AnalogReaders     []mcp3008helper.MCP3008AnalogConfig `json:"analogs,omitempty"`
	DigitalInterrupts []board.DigitalInterruptConfig      `json:"digital_interrupts,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	for idx, c := range conf.AnalogReaders {
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

// LinuxBoardConfig is a struct containing absolutely everything a genericlinux board might need
// configured. It is a union of the configs for the customlinux boards and the genericlinux boards
// with static pin definitions, because those components all use the same underlying code but have
// different config types (e.g., only customlinux can change its pin definitions during
// reconfiguration). The LinuxBoardConfig struct is a unification of the two of them. Whenever we
// go through reconfiguration, we convert the provided config into a LinuxBoardConfig, and then
// reconfigure based on it.
type LinuxBoardConfig struct {
	AnalogReaders     []mcp3008helper.MCP3008AnalogConfig
	DigitalInterrupts []board.DigitalInterruptConfig
	GpioMappings      map[string]GPIOBoardMapping
}

// ConfigConverter is a type synonym for a function to turn whatever config we get during
// reconfiguration into a LinuxBoardConfig, so that we can reconfigure based on that. We return a
// pointer to a LinuxBoardConfig instead of the struct itself so that we can return nil if we
// encounter an error.
type ConfigConverter = func(resource.Config, logging.Logger) (*LinuxBoardConfig, error)

// ConstPinDefs takes in a map from pin names to GPIOBoardMapping structs, and returns a
// ConfigConverter that will use these pin definitions in the underlying config. It is intended to
// be used for board components whose pin definitions are built into the RDK, such as the
// BeagleBone or Jetson boards.
func ConstPinDefs(gpioMappings map[string]GPIOBoardMapping) ConfigConverter {
	return func(conf resource.Config, logger logging.Logger) (*LinuxBoardConfig, error) {
		newConf, err := resource.NativeConfig[*Config](conf)
		if err != nil {
			return nil, err
		}

		return &LinuxBoardConfig{
			AnalogReaders:     newConf.AnalogReaders,
			DigitalInterrupts: newConf.DigitalInterrupts,
			GpioMappings:      gpioMappings,
		}, nil
	}
}

// Package customlinux implements a board running linux
package customlinux

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
)

//lint:ignore U1000 Ignore unused function temporarily
func parsePinConfig(filePath string) ([]genericlinux.PinDefinition, error) {
	pinData, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}

	return parseRawPinData(pinData, filePath)
}

// filePath passed in for logging purposes.
func parseRawPinData(pinData []byte, filePath string) ([]genericlinux.PinDefinition, error) {
	var parsedPinData genericlinux.PinDefinitions
	if err := json.Unmarshal(pinData, &parsedPinData); err != nil {
		return nil, err
	}

	for _, pin := range parsedPinData.Pins {
		err := pin.Validate(filePath)
		if err != nil {
			return nil, err
		}
	}
	return parsedPinData.Pins, nil
}

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	PinConfigFilePath string                         `json:"pin_config_file_path"`
	I2Cs              []board.I2CConfig              `json:"i2cs,omitempty"`
	SPIs              []board.SPIConfig              `json:"spis,omitempty"`
	Analogs           []board.AnalogConfig           `json:"analogs,omitempty"`
	DigitalInterrupts []board.DigitalInterruptConfig `json:"digital_interrupts,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if _, err := os.Stat(conf.PinConfigFilePath); err != nil {
		return nil, err
	}

	boardConfig := genericlinux.Config{
		I2Cs:              conf.I2Cs,
		SPIs:              conf.SPIs,
		Analogs:           conf.Analogs,
		DigitalInterrupts: conf.DigitalInterrupts,
	}
	if deps, err := boardConfig.Validate(path); err != nil {
		return deps, err
	}
	return nil, nil
}

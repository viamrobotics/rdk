// Package customlinux implements a board running linux
package customlinux

import (
	"encoding/json"
	"os"
	"path/filepath"

	"go.viam.com/rdk/components/board/genericlinux"
)

//lint:ignore U1000 Ignore unused function temporarily
func parsePinConfig(filePath string) ([]genericlinux.GenericLinuxPin, error) {
	pinData, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}

	return parseRawPinData(pinData, filePath)
}

// filePath passed in for logging purposes.
func parseRawPinData(pinData []byte, filePath string) ([]genericlinux.GenericLinuxPin, error) {
	var parsedPinData genericlinux.GenericLinuxPins
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
	PinConfigFilePath string `json:"pin_config_filepath"`
	genericlinux.Config
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if _, err := os.Stat(conf.PinConfigFilePath); err != nil {
		return nil, err
	}

	if deps, err := conf.Config.Validate(path); err != nil {
		return deps, err
	}
	return nil, nil
}

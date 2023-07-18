// Package customlinux implements a board running linux
package customlinux

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

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
	var parsedPinData genericlinux.GenericLinuxPins
	if err := json.Unmarshal(pinData, &parsedPinData); err != nil {
		return nil, err
	}

	pinDefs := make([]genericlinux.PinDefinition, len(parsedPinData.Pins))
	for i, pin := range parsedPinData.Pins {
		err := pin.Validate(filePath)
		if err != nil {
			return nil, err
		}

		pinName, err := strconv.Atoi(pin.Name)
		if err != nil {
			return nil, err
		}

		pinDefs[i] = genericlinux.PinDefinition{
			GPIOChipRelativeIDs: map[int]int{pin.Ngpio: pin.LineNumber}, // ngpio: relative id map
			PinNumberBoard:      pinName,
			PWMChipSysFSDir:     pin.PwmChipSysfsDir,
			PWMID:               pin.PwmID,
		}
	}

	return pinDefs, nil
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

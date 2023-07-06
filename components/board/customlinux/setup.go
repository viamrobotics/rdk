package customlinux

import (
	"os"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/board/genericlinux"
)

// GenericLinuxPin describes a gpio pin on a linux board.
type GenericLinuxPin struct {
	Name            string `json:"name"`
	Ngpio           int    `json:"ngpio"`
	RelativeID      int    `json:"relative_id"`
	PWMChipSysFSDir string `json:"pwm_chip_dir,omitempty"`
	PWMID           string `json:"pwm_id,omitempty"`
}

// GenericLinuxPins describes a list of pins on a linux board.
type GenericLinuxPins struct {
	Pins []GenericLinuxPin `json:"pins"`
}

// Validate ensures all parts of the config are valid.
func (config *GenericLinuxPin) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}

	if config.PWMChipSysFSDir != "" && config.PWMID == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "pwm_id")
	}
	return nil
}

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	PinConfigFilePath string              `json:"pin_config_filepath"`
	BoardConfig       genericlinux.Config `json:"board_config,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	if _, err := os.Stat(conf.PinConfigFilePath); err != nil {
		return nil, err
	}

	if _, err := conf.BoardConfig.Validate(path); err != nil {
		return nil, err
	}
	return nil, nil
}

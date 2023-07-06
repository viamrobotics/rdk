// Package customlinux implements a board running linux
package customlinux

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board/genericlinux"
)

// GenericLinuxPin describes a gpio pin on a linux board.
type GenericLinuxPin struct {
	Name            string `json:"name"`
	Ngpio           int    `json:"ngpio"`
	RelativeID      int    `json:"relative_id"`
	PWMChipSysFSDir string `json:"pwm_chip_dir,omitempty"`
	PWMID           int    `json:"pwm_id,omitempty"`
}

// GenericLinuxPins describes a list of pins on a linux board.
type GenericLinuxPins struct {
	Pins []GenericLinuxPin `json:"pins"`
}

// UnmarshalJSON handles setting defaults for pin configs.
func (config *GenericLinuxPin) UnmarshalJSON(text []byte) error {
	type TempPin GenericLinuxPin
	aux := TempPin{
		Ngpio:      -1,
		RelativeID: -1,
		PWMID:      -1,
	}
	if err := json.Unmarshal(text, &aux); err != nil {
		return err
	}
	*config = GenericLinuxPin(aux)
	return nil
}

// Validate ensures all parts of the config are valid.
func (config *GenericLinuxPin) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}

	if config.Ngpio == -1 {
		return utils.NewConfigValidationFieldRequiredError(path, "ngpio")
	}

	if config.RelativeID == -1 {
		return utils.NewConfigValidationFieldRequiredError(path, "relative_id")
	}

	if config.RelativeID > config.Ngpio {
		return utils.NewConfigValidationError(path, errors.New("relative id on gpio chip must be less than ngpio"))
	}

	if config.PWMChipSysFSDir != "" && config.PWMID == -1 {
		return utils.NewConfigValidationError(path, errors.New("must supply pwm_id for the pwm chip"))
	}
	return nil
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

	if _, err := conf.Config.Validate(path); err != nil {
		return nil, err
	}
	return nil, nil
}

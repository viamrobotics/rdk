package genericlinux

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// GPIOBoardMapping represents a GPIO pin's location locally within a GPIO chip
// and globally within sysfs.
type GPIOBoardMapping struct {
	GPIOChipDev    string
	GPIO           int
	GPIOName       string
	PWMSysFsDir    string // Absolute path to the directory, empty string for none
	PWMID          int
	HWPWMSupported bool
}

// PinDefinition describes a gpio pin on a linux board.
type PinDefinition struct {
	Name            string `json:"name"`
	DeviceName      string `json:"device_name"` // name of the pin's chip's device, within /dev
	LineNumber      int    `json:"line_number"` // relative line number on chip
	PwmChipSysfsDir string `json:"pwm_chip_sysfs_dir,omitempty"`
	PwmID           int    `json:"pwm_id,omitempty"`
}

// PinDefinitions describes a list of pins on a linux board.
type PinDefinitions struct {
	Pins []PinDefinition `json:"pins"`
}

// UnmarshalJSON handles setting defaults for pin configs.
// Int values default to -1.
func (conf *PinDefinition) UnmarshalJSON(text []byte) error {
	type TempPin PinDefinition // needed to prevent infinite recursive calls to UnmarshalJSON
	aux := TempPin{
		LineNumber: -1,
		PwmID:      -1,
	}
	if err := json.Unmarshal(text, &aux); err != nil {
		return err
	}
	*conf = PinDefinition(aux)
	return nil
}

// Validate ensures all parts of the config are valid.
func (conf *PinDefinition) Validate(path string) error {
	if conf.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}

	if conf.DeviceName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "device_name")
	}

	if conf.LineNumber == -1 {
		return utils.NewConfigValidationFieldRequiredError(path, "line_number")
	}

	if conf.LineNumber < 0 {
		return utils.NewConfigValidationError(path, errors.New("line_number on gpio chip must be at least zero"))
	}

	if conf.PwmChipSysfsDir != "" && conf.PwmID == -1 {
		return utils.NewConfigValidationError(path, errors.New("must supply pwm_id for the pwm chip"))
	}

	return nil
}

// BoardInformation details pin definitions and device compatibility for a particular board.
type BoardInformation struct {
	PinDefinitions []PinDefinition
	Compats        []string
}

// A NoBoardFoundError is returned when no compatible mapping is found for a board during GPIO board mapping.
type NoBoardFoundError struct {
	modelName string
}

func (err NoBoardFoundError) Error() string {
	return fmt.Sprintf("could not determine %q model", err.modelName)
}

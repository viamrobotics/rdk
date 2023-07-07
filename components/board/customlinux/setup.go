// Package customlinux implements a board running linux
package customlinux

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/utils"
)

// GenericLinuxPin describes a gpio pin on a linux board.
type GenericLinuxPin struct {
	Name            string `json:"name"`
	Ngpio           int    `json:"ngpio"` // this is the ngpio number of the chip the pin is attached to
	RelativeID      int    `json:"relative_id"`
	PwmChipSysfsDir string `json:"pwm_chip_sysfs_dir,omitempty"`
	PwmID           int    `json:"pwm_id,omitempty"`
}

// GenericLinuxPins describes a list of pins on a linux board.
type GenericLinuxPins struct {
	Pins []GenericLinuxPin `json:"pins"`
}

// UnmarshalJSON handles setting defaults for pin configs.
func (conf *GenericLinuxPin) UnmarshalJSON(text []byte) error {
	type TempPin GenericLinuxPin // needed to prevent infinite recursive calls to UnmarshalJSON
	aux := TempPin{
		Ngpio:      -1,
		RelativeID: -1,
		PwmID:      -1,
	}
	if err := json.Unmarshal(text, &aux); err != nil {
		return err
	}
	*conf = GenericLinuxPin(aux)
	return nil
}

// Validate ensures all parts of the config are valid.
func (conf *GenericLinuxPin) Validate(path string) error {
	if conf.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}

	if conf.Ngpio == -1 {
		return utils.NewConfigValidationFieldRequiredError(path, "ngpio")
	}

	if conf.RelativeID == -1 {
		return utils.NewConfigValidationFieldRequiredError(path, "relative_id")
	}

	if conf.RelativeID < 0 {
		return utils.NewConfigValidationError(path, errors.New("relative id on gpio chip must be greater than zero"))
	}

	if conf.RelativeID >= conf.Ngpio {
		return utils.NewConfigValidationError(path, errors.New("relative id on gpio chip must be less than ngpio"))
	}

	if conf.PwmChipSysfsDir != "" && conf.PwmID == -1 {
		return utils.NewConfigValidationError(path, errors.New("must supply pwm_id for the pwm chip"))
	}

	if conf.PwmChipSysfsDir == "" && conf.PwmID != -1 {
		golog.Global().Warnf("No pwm sysfs directory provided for pin %s", conf.Name)
	}

	return nil
}

func parsePinConfig(filePath string) ([]genericlinux.PinDefinition, error) {
	pinData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return parseRawPinData(pinData, filePath)
}

// filePath passed in for logging purposes
func parseRawPinData(pinData []byte, filePath string) ([]genericlinux.PinDefinition, error) {
	var parsedPinData GenericLinuxPins
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
			GPIOChipRelativeIDs: map[int]int{pin.Ngpio: pin.RelativeID}, // ngpio: relative id map
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

	if _, err := conf.Config.Validate(path); err != nil {
		return nil, err
	}
	return nil, nil
}

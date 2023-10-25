//go:build linux

package genericlinux

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.viam.com/rdk/logging"
	rdkutils "go.viam.com/rdk/utils"
)

// adapted from https://github.com/NVIDIA/jetson-gpio (MIT License)

func noBoardError(modelName string) error {
	return fmt.Errorf("could not determine %q model", modelName)
}

// pwmChipData is a struct used solely within GetGPIOBoardMappings and its sub-pieces. It
// describes a PWM chip within sysfs.
type pwmChipData struct {
	Dir  string // Absolute path to pseudofile within sysfs to interact with this chip
	Npwm int    // Taken from the /npwm pseudofile in sysfs: number of lines on the chip
}

// GetGPIOBoardMappings attempts to find a compatible GPIOBoardMapping for the given board.
func GetGPIOBoardMappings(modelName string, boardInfoMappings map[string]BoardInformation) (map[string]GPIOBoardMapping, error) {
	pinDefs, err := getCompatiblePinDefs(modelName, boardInfoMappings)
	if err != nil {
		return nil, err
	}

	return GetGPIOBoardMappingFromPinDefs(pinDefs)
}

// GetGPIOBoardMappingFromPinDefs attempts to find a compatible board-pin mapping using the pin definitions.
func GetGPIOBoardMappingFromPinDefs(pinDefs []PinDefinition) (map[string]GPIOBoardMapping, error) {
	pwmChipsInfo, err := getPwmChipDefs(pinDefs)
	if err != nil {
		// Try continuing on without hardware PWM support. Many boards do not have it enabled by
		// default, and perhaps this robot doesn't even use it.
		logging.Global().Debugw("unable to find PWM chips, continuing without them", "error", err)
		pwmChipsInfo = map[string]pwmChipData{}
	}

	return getBoardMapping(pinDefs, pwmChipsInfo)
}

// getCompatiblePinDefs returns a list of pin definitions, from the first BoardInformation struct
// that appears compatible with the machine we're running on.
func getCompatiblePinDefs(modelName string, boardInfoMappings map[string]BoardInformation) ([]PinDefinition, error) {
	compatibles, err := rdkutils.GetDeviceInfo(modelName)
	if err != nil {
		return nil, fmt.Errorf("error while getting hardware info %w", err)
	}

	var pinDefs []PinDefinition
	for _, info := range boardInfoMappings {
		for _, v := range info.Compats {
			if _, ok := compatibles[v]; ok {
				pinDefs = info.PinDefinitions
				break
			}
		}
	}

	if pinDefs == nil {
		return nil, noBoardError(modelName)
	}
	return pinDefs, nil
}

// A helper function: we read the contents of filePath and return its integer value.
func readIntFile(filePath string) (int, error) {
	//nolint:gosec
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return -1, err
	}
	resultInt64, err := strconv.ParseInt(strings.TrimSpace(string(contents)), 10, 64)
	return int(resultInt64), err
}

func getPwmChipDefs(pinDefs []PinDefinition) (map[string]pwmChipData, error) {
	// First, collect the names of all relevant PWM chips with duplicates removed. Go doesn't have
	// native set objects, so we use a map whose values are ignored.
	pwmChipNames := make(map[string]struct{}, len(pinDefs))
	for _, pinDef := range pinDefs {
		if pinDef.PwmChipSysfsDir == "" {
			continue
		}
		pwmChipNames[pinDef.PwmChipSysfsDir] = struct{}{}
	}

	// Now, look for all chips whose names we found.
	pwmChipsInfo := map[string]pwmChipData{}
	const sysfsDir = "/sys/class/pwm"
	files, err := os.ReadDir(sysfsDir)
	if err != nil {
		return nil, err
	}

	for chipName := range pwmChipNames {
		found := false
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "pwmchip") {
				continue
			}

			// look at symlinks to find the correct chip
			symlink, err := os.Readlink(filepath.Join(sysfsDir, file.Name()))
			if err != nil {
				logging.Global().Errorw(
					"file is not symlink", "file", file.Name(), "err:", err)
				continue
			}

			if strings.Contains(symlink, chipName) {
				found = true
				chipPath := filepath.Join(sysfsDir, file.Name())
				npwm, err := readIntFile(filepath.Join(chipPath, "npwm"))
				if err != nil {
					return nil, err
				}

				pwmChipsInfo[chipName] = pwmChipData{Dir: chipPath, Npwm: npwm}
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("unable to find PWM device %s", chipName)
		}
	}
	return pwmChipsInfo, nil
}

func getBoardMapping(pinDefs []PinDefinition, pwmChipsInfo map[string]pwmChipData,
) (map[string]GPIOBoardMapping, error) {
	data := make(map[string]GPIOBoardMapping, len(pinDefs))

	// For "use" on pins that don't have hardware PWMs
	dummyPwmInfo := pwmChipData{Dir: "", Npwm: -1}

	for _, pinDef := range pinDefs {
		pwmChipInfo, ok := pwmChipsInfo[pinDef.PwmChipSysfsDir]
		if ok {
			if pinDef.PwmID >= pwmChipInfo.Npwm {
				return nil, fmt.Errorf("too high PWM ID %d for pin %s (npwm is %d for chip %s)",
					pinDef.PwmID, pinDef.Name, pwmChipInfo.Npwm, pinDef.PwmChipSysfsDir)
			}
		} else {
			if pinDef.PwmChipSysfsDir == "" {
				// This pin isn't supposed to have hardware PWM support; all is well.
				pwmChipInfo = dummyPwmInfo
			} else {
				logging.Global().Errorw(
					"cannot find expected hardware PWM chip, continuing without it", "pin", pinDef.Name)
				pwmChipInfo = dummyPwmInfo
			}
		}

		data[pinDef.Name] = GPIOBoardMapping{
			GPIOChipDev:    pinDef.DeviceName,
			GPIO:           pinDef.LineNumber,
			GPIOName:       pinDef.Name,
			PWMSysFsDir:    pwmChipInfo.Dir,
			PWMID:          pinDef.PwmID,
			HWPWMSupported: pinDef.PwmID != -1,
		}
	}
	return data, nil
}

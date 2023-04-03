package genericlinux

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// adapted from https://github.com/NVIDIA/jetson-gpio (MIT License)

// GPIOBoardMapping represents a GPIO pin's location locally within a GPIO chip
// and globally within sysfs.
type GPIOBoardMapping struct {
	GPIOChipDev    string
	GPIO           int
	GPIOGlobal     int
	GPIOName       string
	PWMSysFsDir    string
	PWMID          int
	HWPWMSupported bool
}

// PinDefinition describes board specific information on how a particular pin can be accessed
// via sysfs along with information about its PWM capabilities.
type PinDefinition struct {
	GPIOChipRelativeIDs map[int]int    // ngpio -> relative id
	GPIONames           map[int]string // e.g. ngpio=169=PQ.06 for claraAGXXavier
	GPIOChipSysFSDir    string
	PinNumberBoard      int
	PinNumberBCM        int
	PinNameCVM          string
	PinNameTegraSOC     string
	PWMChipSysFSDir     string // empty for none
	PWMID               int    // -1 for none
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

func noBoardError(modelName string) error {
	return fmt.Errorf("could not determine %q model", modelName)
}

// gpioChipData is a struct used solely within GetGPIOBoardMappings and its sub-pieces. It
// describes a GPIO chip within sysfs.
type gpioChipData struct {
	Dir   string // Pseudofile within sysfs to interact with this chip
	Base  int    // Taken from the /base pseudofile in sysfs: offset to the start of the lines
	Ngpio int    // Taken from the /ngpio pseudofile in sysfs: number of lines on the chip
}

// pwmChipData is a struct used solely within GetGPIOBoardMappings and its sub-pieces. It
// describes a PWM chip within sysfs. It has the exact same form as gpioChipData, but we make it a
// separate type so you can't accidentally use one when you should have used the other.
type pwmChipData struct {
	Dir  string // Pseudofile within sysfs to interact with this chip
	Base int    // Taken from the /base pseudofile in sysfs: offset to the start of the lines
	Npwm int    // Taken from the /ngpio pseudofile in sysfs: number of lines on the chip
}

// GetGPIOBoardMappings attempts to find a compatible board-pin mapping for the given mappings.
func GetGPIOBoardMappings(modelName string, boardInfoMappings map[string]BoardInformation) (map[int]GPIOBoardMapping, error) {
	pinDefs, err := getCompatiblePinDefs(modelName, boardInfoMappings)
	if err != nil {
		return nil, err
	}

	gpioChipsInfo, err := getGpioChipDefs(pinDefs)
	if err != nil {
		return nil, err
	}
	pwmChipsInfo, err := getPwmChipDefs(pinDefs)
	if err != nil {
		return nil, err
	}

	return getBoardMapping(pinDefs, gpioChipsInfo, pwmChipsInfo)
}

// getCompatiblePinDefs returns a list of pin definitions, from the first BoardInformation struct
// that appears compatible with the machine we're running on.
func getCompatiblePinDefs(modelName string, boardInfoMappings map[string]BoardInformation) ([]PinDefinition, error) {
	const compatiblePath = "/proc/device-tree/compatible"

	compatiblesRd, err := os.ReadFile(compatiblePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, noBoardError(modelName)
		}
		return nil, err
	}
	compatibles := utils.NewStringSet(strings.Split(string(compatiblesRd), "\x00")...)

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

func getGpioChipDefs(pinDefs []PinDefinition) (map[string]gpioChipData, error) {
	gpioChipsInfo := map[string]gpioChipData{}
	sysfsPrefixes := []string{"/sys/devices/", "/sys/devices/platform/", "/sys/devices/platform/bus@100000/"}

	// Get a set of all the chip names with duplicates removed. Go doesn't have native set objects,
	// so we use a map whose values are ignored.
	gpioChipNames := make(map[string]struct{}, len(pinDefs))
	for _, pinDef := range pinDefs {
		if pinDef.GPIOChipSysFSDir == "" {
			continue
		}
		gpioChipNames[pinDef.GPIOChipSysFSDir] = struct{}{}
	}

	for gpioChipName := range gpioChipNames {
		var gpioChipDir string
		for _, prefix := range sysfsPrefixes {
			d := prefix + gpioChipName
			fileInfo, err := os.Stat(d)
			if err != nil {
				continue
			}
			if fileInfo.IsDir() {
				gpioChipDir = d
				break
			}
		}
		if gpioChipDir == "" {
			return nil, errors.Errorf("cannot find GPIO chip %q", gpioChipName)
		}
		files, err := os.ReadDir(gpioChipDir)
		if err != nil {
			return nil, err
		}
		var chipFileName string
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "gpiochip") {
				continue
			}
			chipFileName = file.Name()
			break
		}

		gpioChipGPIODir := gpioChipDir + "/gpio"
		files, err = os.ReadDir(gpioChipGPIODir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "gpiochip") {
				continue
			}

			base, err := readIntFile(filepath.Join(gpioChipGPIODir, file.Name(), "base"))
			if err != nil {
				return nil, err
			}

			ngpio, err := readIntFile(filepath.Join(gpioChipGPIODir, file.Name(), "ngpio"))
			if err != nil {
				return nil, err
			}

			gpioChipsInfo[gpioChipName] = gpioChipData{
				Dir:   chipFileName,
				Base:  base,
				Ngpio: ngpio,
			}
			break
		}
	}

	return gpioChipsInfo, nil
}

func getPwmChipDefs(pinDefs []PinDefinition) (map[string]pwmChipData, error) {
	// First, collect the names of all relevant PWM chips with duplicates removed. Go doesn't have
	// native set objects, so we use a map whose values are ignored.
	pwmChipNames := make(map[string]struct{}, len(pinDefs))
	for _, pinDef := range pinDefs {
		if pinDef.PWMChipSysFSDir == "" {
			continue
		}
		pwmChipNames[pinDef.PWMChipSysFSDir] = struct{}{}
	}

	// Now, look for all chips whose names we found.
	pwmChipsInfo := map[string]pwmChipData{}
	for chipName := range pwmChipNames {
		chipDir := fmt.Sprintf("/sys/devices/platform/%s/pwm", chipName)
		// There should be a single directory within /sys/devices/platform/<chipName>/pwm/, whose
		// name is mirrored in /sys/class/pwm. That's the one we want to use.
		// TODO[RSDK-2332]: make this universally usable by all genericlinux boards.
		files, err := os.ReadDir(chipDir)
		if err != nil {
			return nil, err
		}

		found := false
		for _, file := range files {
			if !(strings.Contains(file.Name(), "pwmchip") && file.IsDir()) {
				continue
			}
			found = true
			chipPath := fmt.Sprintf("/sys/class/pwm/%s", file.Name())
			baseInt := -1 // TODO: implement these.
			npwmInt := -1
			pwmChipsInfo[chipName] = pwmChipData{Dir: chipPath, Base: baseInt, Npwm: npwmInt}
			break
		}
		if !found {
			return nil, fmt.Errorf("unable to find PWM device %s", chipName)
		}
	}
	return pwmChipsInfo, nil
}

func getBoardMapping(pinDefs []PinDefinition, gpioChipsInfo map[string]gpioChipData, pwmChipsInfo map[string]pwmChipData) (map[int]GPIOBoardMapping, error) {
	data := make(map[int]GPIOBoardMapping, len(pinDefs))

	// For "use" on pins that don't have hardware PWMs
	dummyPwmInfo := pwmChipData{Dir: "", Base: 0, Npwm: -1}

	for _, pinDef := range pinDefs {
		key := pinDef.PinNumberBoard

		gpioChipInfo, ok := gpioChipsInfo[pinDef.GPIOChipSysFSDir]
		if !ok {
			return nil, fmt.Errorf("unknown GPIO device %s for pin %d",
				pinDef.GPIOChipSysFSDir, key)
		}

		pwmChipInfo, ok := pwmChipsInfo[pinDef.PWMChipSysFSDir]
		if ok {
			if pinDef.PWMID >= pwmChipInfo.Npwm {
				return nil, fmt.Errorf("too high PWM ID %s for pin %d",
					pinDef.PWMID, key)
			}
		} else {
			if pinDef.PWMChipSysFSDir == "" {
				// This pin isn't supposed to have hardware PWM support; all is well.
				pwmChipInfo = dummyPwmInfo
			} else {
				return nil, fmt.Errorf("unknown PWM device %s for pin %d",
					pinDef.GPIOChipSysFSDir, key)
			}
		}

		chipRelativeID, ok := pinDef.GPIOChipRelativeIDs[gpioChipInfo.Ngpio]
		if !ok {
			chipRelativeID = pinDef.GPIOChipRelativeIDs[-1]
		}

		data[key] = GPIOBoardMapping{
			GPIOChipDev:    gpioChipInfo.Dir,
			GPIO:           chipRelativeID,
			GPIOGlobal:     gpioChipInfo.Base + chipRelativeID,
			GPIOName:       pinDef.PinNameCVM,
			PWMSysFsDir:    pwmChipInfo.Dir,
			PWMID:          pwmChipInfo.Base + pinDef.PWMID,
			HWPWMSupported: pinDef.PWMID != -1,
		}
	}
	return data, nil
}

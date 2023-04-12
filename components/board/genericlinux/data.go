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
	PWMSysFsDir    string // Absolute path to the directory, empty string for none
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
// describes a PWM chip within sysfs.
type pwmChipData struct {
	Dir  string // Absolute path to pseudofile within sysfs to interact with this chip
	Npwm int    // Taken from the /npwm pseudofile in sysfs: number of lines on the chip
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

	mapping, err := getBoardMapping(pinDefs, gpioChipsInfo, pwmChipsInfo)
	return mapping, err
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
		found := false

		// The boards we support might store their PWM devices in several different locations
		// within /sys/devices. If it turns out that every board stores its PWM chips in a
		// different location, we should rethink this. Maybe a breadth-first search over all of
		// /sys/devices/? but for now, this is good enough.
		directoriesToSearch := []string{
			// Example boards which use each location:
			// Jetson Orin AGX       BeagleBone AI64                     Intel UP 4000
			"/sys/devices/platform", "/sys/devices/platform/bus@100000", "/sys/devices/pci0000:00",
		}
		for _, baseDir := range directoriesToSearch {
			// For exactly one baseDir, there should be a directory at <baseDir>/<chipName>/pwm/,
			// which contains a single sub-directory whose name is mirrored in /sys/class/pwm.
			// That's the one we want to use.
			chipDir := fmt.Sprintf("%s/%s/pwm", baseDir, chipName)
			files, err := os.ReadDir(chipDir)
			if err != nil {
				continue // This was the wrong directory; try the next baseDir.
			}

			// We've found what looks like the right place to look for things! Now, find the name
			// of the chip that should be mirrored in /sys/class/pwm/.
			for _, file := range files {
				if !(strings.Contains(file.Name(), "pwmchip") && file.IsDir()) {
					continue
				}
				found = true
				chipPath := fmt.Sprintf("/sys/class/pwm/%s", file.Name())

				npwm, err := readIntFile(filepath.Join(chipPath, "npwm"))
				if err != nil {
					return nil, err
				}

				pwmChipsInfo[chipName] = pwmChipData{Dir: chipPath, Npwm: npwm}
				// Now that we've found the chip info, we need to break out of 2 different for
				// loops, to go on to the next chip name. This is just the first one so far...
				break
			}
			if found {
				// ...and this is the second one. We've already found the info for the current
				// chip name, so move on to the next name.
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unable to find PWM device %s", chipName)
		}
	}
	return pwmChipsInfo, nil
}

func getBoardMapping(pinDefs []PinDefinition, gpioChipsInfo map[string]gpioChipData,
	pwmChipsInfo map[string]pwmChipData,
) (map[int]GPIOBoardMapping, error) {
	data := make(map[int]GPIOBoardMapping, len(pinDefs))

	// For "use" on pins that don't have hardware PWMs
	dummyPwmInfo := pwmChipData{Dir: "", Npwm: -1}

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
				return nil, fmt.Errorf("too high PWM ID %d for pin %d (npwm is %d for chip %s)",
					pinDef.PWMID, key, pwmChipInfo.Npwm, pinDef.PWMChipSysFSDir)
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
			PWMID:          pinDef.PWMID,
			HWPWMSupported: pinDef.PWMID != -1,
		}
	}
	return data, nil
}

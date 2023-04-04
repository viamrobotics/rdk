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

// GetGPIOBoardMappings attempts to find a compatible board-pin mapping for the given mappings.
func GetGPIOBoardMappings(modelName string, boardInfoMappings map[string]BoardInformation) (map[int]GPIOBoardMapping, error) {
	pinDefs, err := getCompatiblePinDefs(modelName, boardInfoMappings)
	if err != nil {
		return nil, err
	}

	gpioChipInfo, err := getGpioChipDefs(pinDefs)
	if err != nil {
		return nil, err
	}

	return getBoardMapping(pinDefs, gpioChipInfo)
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

func getGpioChipDefs(pinDefs []PinDefinition) (map[string]gpioChipData, error) {
	gpioChipInfo := map[string]gpioChipData{}
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

			baseFn := filepath.Join(gpioChipGPIODir, file.Name(), "base")
			//nolint:gosec
			baseRd, err := os.ReadFile(baseFn)
			if err != nil {
				return nil, err
			}
			baseParsed, err := strconv.ParseInt(strings.TrimSpace(string(baseRd)), 10, 64)
			if err != nil {
				return nil, err
			}

			ngpioFn := filepath.Join(gpioChipGPIODir, file.Name(), "ngpio")
			//nolint:gosec
			ngpioRd, err := os.ReadFile(ngpioFn)
			if err != nil {
				return nil, err
			}
			ngpioParsed, err := strconv.ParseInt(strings.TrimSpace(string(ngpioRd)), 10, 64)
			if err != nil {
				return nil, err
			}

			gpioChipInfo[gpioChipName] = gpioChipData{
				Dir:   chipFileName,
				Base:  int(baseParsed),
				Ngpio: int(ngpioParsed),
			}
			break
		}
	}

	return gpioChipInfo, nil
}

func getBoardMapping(pinDefs []PinDefinition, gpioChipInfo map[string]gpioChipData) (map[int]GPIOBoardMapping, error) {
	data := make(map[int]GPIOBoardMapping, len(pinDefs))

	for _, pinDef := range pinDefs {
		key := pinDef.PinNumberBoard

		chipInfo, ok := gpioChipInfo[pinDef.GPIOChipSysFSDir]
		if !ok {
			return nil, fmt.Errorf("unknown GPIO device %s for pin %d",
				pinDef.GPIOChipSysFSDir, key)
		}

		chipRelativeID, ok := pinDef.GPIOChipRelativeIDs[chipInfo.Ngpio]
		if !ok {
			chipRelativeID = pinDef.GPIOChipRelativeIDs[-1]
		}

		data[key] = GPIOBoardMapping{
			GPIOChipDev:    chipInfo.Dir,
			GPIO:           chipRelativeID,
			GPIOGlobal:     chipInfo.Base + chipRelativeID,
			GPIOName:       pinDef.PinNameCVM,
			PWMSysFsDir:    pinDef.PWMChipSysFSDir,
			PWMID:          pinDef.PWMID,
			HWPWMSupported: pinDef.PWMID != -1,
		}
	}
	return data, nil
}

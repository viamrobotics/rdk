package commonsysfs

import (
	"fmt"
	"io/ioutil"
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

// GetGPIOBoardMappings attempts to find a compatible board-pin mapping for the given mappings.
func GetGPIOBoardMappings(modelName string, boardInfoMappings map[string]BoardInformation) (map[int]GPIOBoardMapping, error) {
	const (
		compatiblePath = "/proc/device-tree/compatible"
		idsPath        = "/proc/device-tree/chosen/plugin-manager/ids"
	)

	compatiblesRd, err := ioutil.ReadFile(compatiblePath)
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

	gpioChipDirs := map[string]string{}
	gpioChipBase := map[string]int{}
	gpioChipNgpio := map[string]int{}

	sysfsPrefixes := []string{"/sys/devices/", "/sys/devices/platform/", "/sys/devices/platform/bus@100000/"}

	// Get the GPIO chip offsets
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
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "gpiochip") {
				continue
			}
			gpioChipDirs[gpioChipName] = file.Name()
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
			baseRd, err := ioutil.ReadFile(baseFn)
			if err != nil {
				return nil, err
			}
			baseParsed, err := strconv.ParseInt(strings.TrimSpace(string(baseRd)), 10, 64)
			if err != nil {
				return nil, err
			}
			gpioChipBase[gpioChipName] = int(baseParsed)

			ngpioFn := filepath.Join(gpioChipGPIODir, file.Name(), "ngpio")
			//nolint:gosec
			ngpioRd, err := ioutil.ReadFile(ngpioFn)
			if err != nil {
				return nil, err
			}
			ngpioParsed, err := strconv.ParseInt(strings.TrimSpace(string(ngpioRd)), 10, 64)
			if err != nil {
				return nil, err
			}
			gpioChipNgpio[gpioChipName] = int(ngpioParsed)
			break
		}
	}

	data := make(map[int]GPIOBoardMapping, len(pinDefs))
	for _, pinDef := range pinDefs {
		key := pinDef.PinNumberBoard

		chipGPIONgpio := gpioChipNgpio[pinDef.GPIOChipSysFSDir]
		chipGPIOBase := gpioChipBase[pinDef.GPIOChipSysFSDir]
		chipRelativeID, ok := pinDef.GPIOChipRelativeIDs[chipGPIONgpio]
		if !ok {
			chipRelativeID = pinDef.GPIOChipRelativeIDs[-1]
		}

		data[key] = GPIOBoardMapping{
			GPIOChipDev:    gpioChipDirs[pinDef.GPIOChipSysFSDir],
			GPIO:           chipRelativeID,
			GPIOGlobal:     chipGPIOBase + chipRelativeID,
			HWPWMSupported: pinDef.PWMID != -1,
		}
	}

	return data, nil
}

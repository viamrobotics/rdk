//go:build linux

package genericlinux

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/mkch/gpio"
	rdkutils "go.viam.com/rdk/utils"
)

// adapted from https://github.com/NVIDIA/jetson-gpio (MIT License)

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
		// Try continuing on without hardware PWM support. Many boards do not have it enabled by
		// default, and perhaps this robot doesn't even use it.
		golog.Global().Debugw("unable to find PWM chips, continuing without them", "error", err)
		pwmChipsInfo = map[string]pwmChipData{}
	}

	mapping, err := getBoardMapping(pinDefs, gpioChipsInfo, pwmChipsInfo)
	return mapping, err
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

func getGpioChipDefs(pinDefs []PinDefinition) (map[int]gpioChipData, error) {
	gpioChipsInfo := map[int]gpioChipData{}

	allDevices := gpio.ChipDevices()
	gpioChipNgpios := make(map[int]string, len(allDevices)) // maps chipNgpio -> string gpiochip#
	for _, dev := range allDevices {
		chip, err := gpio.OpenChip(dev)
		if err != nil {
			return nil, err
		}

		chipInfo, err := chip.Info()
		if err != nil {
			return nil, err
		}

		gpioChipNgpios[int(chipInfo.NumLines)] = chipInfo.Name
	}

	gpioConfigNgpios := make(map[int]struct{}, len(pinDefs))
	for _, pinDef := range pinDefs {
		for n := range pinDef.GPIOChipRelativeIDs {
			gpioConfigNgpios[n] = struct{}{} // get a "set" of all ngpio numbers on the board
		}
	}

	// TODO: remove this and base attribute after periph removed
	const sysfsPrefix := "/sys/class/gpio"
	sysfsFiles, err := os.ReadDir(sysfsPrefix)
	if err != nil {
		return nil, err
	}

	// for each chip in the board config, find the right gpioChip dir
	for chipNgpio := range gpioConfigNgpios { 
		var base int
		for _, file := range sysfsFiles {
			// code looks through sys/class/gpio to find the base offset of the chip
			// TODO: remove this once periph is removed
			if !strings.HasPrefix(file.Name(), "gpiochip") { // files should have format gpioChip#
				continue
			}

			ngpio, err := readIntFile(filepath.Join(sysfsPrefix, file.Name(), "ngpio")) // read from /sys/class/gpio/gpiochip#/ngpio
			if err != nil {
				return nil, err
			}

			if ngpio != chipNgpio {
				continue
			}

			base, err = readIntFile(filepath.Join(sysfsPrefix, file.Name(), "base")) // read from /sys/class/gpio/gpiochip#/base
			if err != nil {
				return nil, err
			}
			break
		}

		dir, ok := gpioChipNgpios[chipNgpio]

		if !ok {
			return nil, fmt.Errorf("unknown GPIO device with ngpio %d",
				chipNgpio)
		}

		gpioChipsInfo[chipNgpio] = gpioChipData{
			Dir:   dir,
			Base:  base,
			Ngpio: chipNgpio,
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

func getBoardMapping(pinDefs []PinDefinition, gpioChipsInfo map[int]gpioChipData,
	pwmChipsInfo map[string]pwmChipData,
) (map[int]GPIOBoardMapping, error) {
	data := make(map[int]GPIOBoardMapping, len(pinDefs))

	// For "use" on pins that don't have hardware PWMs
	dummyPwmInfo := pwmChipData{Dir: "", Npwm: -1}

	for _, pinDef := range pinDefs {
		key := pinDef.PinNumberBoard

		var ngpio int
		for n := range pinDef.GPIOChipRelativeIDs {
			ngpio = n
			break // each gpio pin should only be associated with one gpiochip in the config
		}

		gpioChipInfo, ok := gpioChipsInfo[ngpio]
		if !ok {
			return nil, fmt.Errorf("unknown GPIO device for chip with ngpio %d, pin %d",
				ngpio, key)
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
				golog.Global().Errorw(
					"cannot find expected hardware PWM chip, continuing without it", "pin", key)
				pwmChipInfo = dummyPwmInfo
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

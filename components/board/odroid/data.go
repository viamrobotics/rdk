package odroid

import (
	"go.viam.com/rdk/components/board/genericlinux"
)

const c4 = "ODROID-C4"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	// pins 11, 15, and 35 are not included in the mapping beacause trying to use them at the same time as
	// the pin on the same pwm chip (pins 7, 12, 33 respectively) causes errors. Only one pin from each
	// pwm chip is included in the board mapping for simplicity.
	c4: {
		PinDefinitions: []genericlinux.PinDefinition{
			{Name: "3", DeviceName: "gpiochip1", LineNumber: 83, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", DeviceName: "gpiochip1", LineNumber: 84, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "7", DeviceName: "gpiochip1", LineNumber: 71, PwmChipSysfsDir: "ffd1a000.pwm", PwmID: 0},
			{Name: "8", DeviceName: "gpiochip1", LineNumber: 78, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", DeviceName: "gpiochip1", LineNumber: 79, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", DeviceName: "gpiochip1", LineNumber: 82, PwmChipSysfsDir: "ffd19000.pwm", PwmID: 0},
			{Name: "13", DeviceName: "gpiochip1", LineNumber: 70, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", DeviceName: "gpiochip1", LineNumber: 66, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", DeviceName: "gpiochip1", LineNumber: 67, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", DeviceName: "gpiochip1", LineNumber: 74, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", DeviceName: "gpiochip1", LineNumber: 75, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", DeviceName: "gpiochip1", LineNumber: 68, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", DeviceName: "gpiochip1", LineNumber: 77, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", DeviceName: "gpiochip1", LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", DeviceName: "gpiochip1", LineNumber: 23, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "27", DeviceName: "gpiochip1", LineNumber: 64, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "28", DeviceName: "gpiochip1", LineNumber: 65, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "29", DeviceName: "gpiochip1", LineNumber: 80, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "31", DeviceName: "gpiochip1", LineNumber: 81, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "32", DeviceName: "gpiochip1", LineNumber: 24, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "33", DeviceName: "gpiochip1", LineNumber: 72, PwmChipSysfsDir: "ffd1b000.pwm", PwmID: 0},
			{Name: "36", DeviceName: "gpiochip1", LineNumber: 22, PwmChipSysfsDir: "", PwmID: -1},
		},
		Compats: []string{"amlogic, g12a"},
	},
}

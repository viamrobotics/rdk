package ti

import "go.viam.com/rdk/components/board/genericlinux"

const tiTDA4VM = "ti_tda4vm"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	tiTDA4VM: {
		[]genericlinux.PinDefinition{
			// Pins 3 and 5 don't work as GPIO by default; you might need to disable the I2C bus to
			// use them.
			{Name: "3", DeviceName: "gpiochip1", LineNumber: 84, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", DeviceName: "gpiochip1", LineNumber: 83, PwmChipSysfsDir: "", PwmID: -1},
			// Pin 7 appears to be input-only, due to some sort of hardware limitation.
			{Name: "7", DeviceName: "gpiochip1", LineNumber: 7, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "8", DeviceName: "gpiochip1", LineNumber: 70, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", DeviceName: "gpiochip1", LineNumber: 81, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", DeviceName: "gpiochip1", LineNumber: 71, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", DeviceName: "gpiochip1", LineNumber: 1, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", DeviceName: "gpiochip1", LineNumber: 82, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", DeviceName: "gpiochip1", LineNumber: 11, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", DeviceName: "gpiochip1", LineNumber: 5, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", DeviceName: "gpiochip2", LineNumber: 12, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", DeviceName: "gpiochip1", LineNumber: 101, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", DeviceName: "gpiochip1", LineNumber: 107, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", DeviceName: "gpiochip1", LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", DeviceName: "gpiochip1", LineNumber: 103, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", DeviceName: "gpiochip1", LineNumber: 102, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", DeviceName: "gpiochip1", LineNumber: 108, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "29", DeviceName: "gpiochip1", LineNumber: 93, PwmChipSysfsDir: "3020000.pwm", PwmID: 0},
			{Name: "31", DeviceName: "gpiochip1", LineNumber: 94, PwmChipSysfsDir: "3020000.pwm", PwmID: 1},
			{Name: "32", DeviceName: "gpiochip1", LineNumber: 98, PwmChipSysfsDir: "3030000.pwm", PwmID: 0},
			{Name: "33", DeviceName: "gpiochip1", LineNumber: 99, PwmChipSysfsDir: "3030000.pwm", PwmID: 1},
			{Name: "35", DeviceName: "gpiochip1", LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "36", DeviceName: "gpiochip1", LineNumber: 97, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "37", DeviceName: "gpiochip1", LineNumber: 115, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "38", DeviceName: "gpiochip1", LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "40", DeviceName: "gpiochip1", LineNumber: 4, PwmChipSysfsDir: "", PwmID: -1},
		},
		[]string{"ti,j721e-sk", "ti,j721e"},
	},
}

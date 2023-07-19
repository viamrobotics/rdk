package ti

import "go.viam.com/rdk/components/board/genericlinux"

const tiTDA4VM = "ti_tda4vm"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	tiTDA4VM: {
		[]genericlinux.PinDefinition{
			// Pins 3 and 5 don't work as GPIO by default; you might need to disable the I2C bus to
			// use them.
			{Name: "3", Ngpio: 128, LineNumber: 84, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", Ngpio: 128, LineNumber: 83, PwmChipSysfsDir: "", PwmID: -1},
			// Pin 7 appears to be input-only, due to some sort of hardware limitation.
			{Name: "7", Ngpio: 128, LineNumber: 7, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "8", Ngpio: 128, LineNumber: 70, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", Ngpio: 128, LineNumber: 81, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", Ngpio: 128, LineNumber: 71, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", Ngpio: 128, LineNumber: 1, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", Ngpio: 128, LineNumber: 82, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", Ngpio: 128, LineNumber: 11, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", Ngpio: 128, LineNumber: 5, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", Ngpio: 36, LineNumber: 12, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", Ngpio: 128, LineNumber: 101, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", Ngpio: 128, LineNumber: 107, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", Ngpio: 128, LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", Ngpio: 128, LineNumber: 103, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", Ngpio: 128, LineNumber: 102, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", Ngpio: 128, LineNumber: 108, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "29", Ngpio: 128, LineNumber: 93, PwmChipSysfsDir: "3020000.pwm", PwmID: 0},
			{Name: "31", Ngpio: 128, LineNumber: 94, PwmChipSysfsDir: "3020000.pwm", PwmID: 1},
			{Name: "32", Ngpio: 128, LineNumber: 98, PwmChipSysfsDir: "3030000.pwm", PwmID: 0},
			{Name: "33", Ngpio: 128, LineNumber: 99, PwmChipSysfsDir: "3030000.pwm", PwmID: 1},
			{Name: "35", Ngpio: 128, LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "36", Ngpio: 128, LineNumber: 97, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "37", Ngpio: 128, LineNumber: 115, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "38", Ngpio: 128, LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "40", Ngpio: 128, LineNumber: 4, PwmChipSysfsDir: "", PwmID: -1},
		},
		[]string{"ti,j721e-sk", "ti,j721e"},
	},
}

package upboard

// This is experimental.

import "go.viam.com/rdk/components/board/genericlinux"

const upboard = "up_4000"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	upboard: {
		[]genericlinux.PinDefinition{
			/*
				pinout for up4000: https://github.com/up-board/up-community/wiki/Pinout_UP4000
				GPIOChipSysFSDir: path to the directory of a chip. Can be found from the output of gpiodetect
			*/
			// GPIO pin definition
			{Name: "3", Ngpio: 28, LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", Ngpio: 28, LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "7", Ngpio: 28, LineNumber: 4, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "8", Ngpio: 28, LineNumber: 14, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", Ngpio: 28, LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", Ngpio: 28, LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", Ngpio: 28, LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", Ngpio: 28, LineNumber: 27, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", Ngpio: 28, LineNumber: 22, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", Ngpio: 28, LineNumber: 23, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 3},
			{Name: "18", Ngpio: 28, LineNumber: 24, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", Ngpio: 28, LineNumber: 10, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", Ngpio: 28, LineNumber: 9, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", Ngpio: 28, LineNumber: 25, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", Ngpio: 28, LineNumber: 11, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", Ngpio: 28, LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", Ngpio: 28, LineNumber: 7, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "27", Ngpio: 28, LineNumber: 0, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "28", Ngpio: 28, LineNumber: 1, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "29", Ngpio: 28, LineNumber: 5, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "31", Ngpio: 28, LineNumber: 6, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "32", Ngpio: 28, LineNumber: 12, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 1},
			{Name: "33", Ngpio: 28, LineNumber: 13, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 0},
			{Name: "35", Ngpio: 28, LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "36", Ngpio: 28, LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "37", Ngpio: 28, LineNumber: 26, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "38", Ngpio: 28, LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "40", Ngpio: 28, LineNumber: 21, PwmChipSysfsDir: "", PwmID: -1},
		},
		[]string{"UP-APL03"},
	},
}

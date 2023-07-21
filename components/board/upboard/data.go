package upboard

// This is experimental.

import "go.viam.com/rdk/components/board/genericlinux"

const upboard = "up_4000"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	upboard: {
		[]genericlinux.PinDefinition{
			/*
				pinout for up4000: https://github.com/up-board/up-community/wiki/Pinout_UP4000
				GPIOChipRelativeIDs: {ngpio : base-linux_gpio_number}
				GPIOChipSysFSDir: path to the directory of a chip. Can be found from the output of gpiodetect
			*/
			// GPIO pin definition
			{Name: "29", Ngpio: 78, LineNumber: 73, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "31", Ngpio: 77, LineNumber: 46, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", Ngpio: 77, LineNumber: 48, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", Ngpio: 77, LineNumber: 45, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "37", Ngpio: 77, LineNumber: 46, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "35", Ngpio: 47, LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", Ngpio: 77, LineNumber: 75, PwmChipSysfsDir: "", PwmID: -1},

			// ttyS4 UART
			{Name: "8", Ngpio: 78, LineNumber: 43, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", Ngpio: 78, LineNumber: 42, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", Ngpio: 78, LineNumber: 44, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "36", Ngpio: 78, LineNumber: 45, PwmChipSysfsDir: "", PwmID: -1},

			// I2c
			{Name: "3", Ngpio: 78, LineNumber: 28, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", Ngpio: 78, LineNumber: 29, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "28", Ngpio: 78, LineNumber: 31, PwmChipSysfsDir: "", PwmID: -1},

			// pwm
			{Name: "33", Ngpio: 78, LineNumber: 35, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 0},
			{Name: "32", Ngpio: 78, LineNumber: 34, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 1},
			{Name: "16", Ngpio: 78, LineNumber: 37, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 3},
			{Name: "7", Ngpio: 77, LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", Ngpio: 77, LineNumber: 65, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", Ngpio: 77, LineNumber: 64, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", Ngpio: 77, LineNumber: 61, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "27", Ngpio: 78, LineNumber: 30, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", Ngpio: 47, LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", Ngpio: 77, LineNumber: 62, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", Ngpio: 77, LineNumber: 63, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "38", Ngpio: 47, LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "40", Ngpio: 47, LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", Ngpio: 77, LineNumber: 74, PwmChipSysfsDir: "", PwmID: -1},
		},
		[]string{"UP-APL03"},
	},
}

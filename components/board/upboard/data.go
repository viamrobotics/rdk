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
			{Name: "3", DeviceName: "gpiochip4", LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", DeviceName: "gpiochip4", LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "7", DeviceName: "gpiochip4", LineNumber: 4, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "8", DeviceName: "gpiochip4", LineNumber: 14, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", DeviceName: "gpiochip4", LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", DeviceName: "gpiochip4", LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", DeviceName: "gpiochip4", LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", DeviceName: "gpiochip4", LineNumber: 27, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", DeviceName: "gpiochip4", LineNumber: 22, PwmChipSysfsDir: "", PwmID: -1},
			// Pin 16 supposedly has hardware PWM from pwmID 3, but we haven't gotten it to work.
			{Name: "16", DeviceName: "gpiochip4", LineNumber: 23, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", DeviceName: "gpiochip4", LineNumber: 24, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", DeviceName: "gpiochip4", LineNumber: 10, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", DeviceName: "gpiochip4", LineNumber: 9, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", DeviceName: "gpiochip4", LineNumber: 25, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", DeviceName: "gpiochip4", LineNumber: 11, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", DeviceName: "gpiochip4", LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", DeviceName: "gpiochip4", LineNumber: 7, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "27", DeviceName: "gpiochip4", LineNumber: 0, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "28", DeviceName: "gpiochip4", LineNumber: 1, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "29", DeviceName: "gpiochip4", LineNumber: 5, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "31", DeviceName: "gpiochip4", LineNumber: 6, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "32", DeviceName: "gpiochip4", LineNumber: 12, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 0},
			{Name: "33", DeviceName: "gpiochip4", LineNumber: 13, PwmChipSysfsDir: "0000:00:1a.0", PwmID: 1},
			{Name: "35", DeviceName: "gpiochip4", LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "36", DeviceName: "gpiochip4", LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "37", DeviceName: "gpiochip4", LineNumber: 26, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "38", DeviceName: "gpiochip4", LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "40", DeviceName: "gpiochip4", LineNumber: 21, PwmChipSysfsDir: "", PwmID: -1},
		},
		[]string{"UP-APL03"},
	},
}

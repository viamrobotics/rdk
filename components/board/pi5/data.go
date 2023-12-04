package pi5

import "go.viam.com/rdk/components/board/genericlinux"

// Thanks to "Dan Makes Things" at https://www.makerforge.tech/posts/viam-custom-board-pi5/ for
// collaborating on setting this up!
var boardInfoMappings = map[string]genericlinux.BoardInformation{
	"pi5": {
		PinDefinitions: []genericlinux.PinDefinition{
			{Name: "3", DeviceName: "gpiochip4", LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", DeviceName: "gpiochip4", LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "7", DeviceName: "gpiochip4", LineNumber: 4, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "8", DeviceName: "gpiochip4", LineNumber: 14, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", DeviceName: "gpiochip4", LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", DeviceName: "gpiochip4", LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", DeviceName: "gpiochip4", LineNumber: 18, PwmChipSysfsDir: "1f00098000.pwm", PwmID: 2},
			{Name: "13", DeviceName: "gpiochip4", LineNumber: 27, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", DeviceName: "gpiochip4", LineNumber: 22, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", DeviceName: "gpiochip4", LineNumber: 23, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", DeviceName: "gpiochip4", LineNumber: 24, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", DeviceName: "gpiochip4", LineNumber: 10, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", DeviceName: "gpiochip4", LineNumber: 9, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", DeviceName: "gpiochip4", LineNumber: 25, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", DeviceName: "gpiochip4", LineNumber: 11, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", DeviceName: "gpiochip4", LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", DeviceName: "gpiochip4", LineNumber: 7, PwmChipSysfsDir: "", PwmID: -1},
			// Per https://www.raspberrypi.com/documentation/computers/images/GPIO-duplicate.png
			// Physical pins 27 and 28 (shown in white in that diagram) should not be used for
			// normal GPIO stuff.
			{Name: "29", DeviceName: "gpiochip4", LineNumber: 5, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "31", DeviceName: "gpiochip4", LineNumber: 6, PwmChipSysfsDir: "", PwmID: -1},
			// We'd expect pins 32 and 33 to have hardware PWM support, too, but we haven't gotten
			// that to work yet.
			{Name: "32", DeviceName: "gpiochip4", LineNumber: 12, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "33", DeviceName: "gpiochip4", LineNumber: 13, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "35", DeviceName: "gpiochip4", LineNumber: 19, PwmChipSysfsDir: "1f00098000.pwm", PwmID: 3},
			{Name: "36", DeviceName: "gpiochip4", LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "37", DeviceName: "gpiochip4", LineNumber: 26, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "38", DeviceName: "gpiochip4", LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "40", DeviceName: "gpiochip4", LineNumber: 21, PwmChipSysfsDir: "", PwmID: -1},
		},
		Compats: []string{"raspberrypi,5-model-b", "brcm,bcm2712"},
	},
}

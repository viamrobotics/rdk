package orangepi

import "go.viam.com/rdk/components/board/genericlinux"

const OPzero2 = "OrangePi Zero2"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	OPzero2: {
		PinDefinitions: []genericlinux.PinDefinition{
			{Name: "3", DeviceName: "gpiochip0", LineNumber: 229},
			{Name: "5", DeviceName: "gpiochip0", LineNumber: 228},
			{Name: "7", DeviceName: "gpiochip0", LineNumber: 73},
			{Name: "11", DeviceName: "gpiochip0", LineNumber: 70},
			{Name: "13", DeviceName: "gpiochip0", LineNumber: 69},
			{Name: "15", DeviceName: "gpiochip0", LineNumber: 72},
			{Name: "19", DeviceName: "gpiochip0", LineNumber: 231},
			{Name: "21", DeviceName: "gpiochip0", LineNumber: 232},
			// When we can switch between gpio and pwm, this would have line number 226.
			{Name: "8", DeviceName: "gpiochip0", LineNumber: -1, PwmChipSysfsDir: "300a000.pwm", PwmID: 2},
			// When we can switch between gpio and pwm, this would have line number 227.
			{Name: "10", DeviceName: "gpiochip0", LineNumber: -1, PwmChipSysfsDir: "300a000.pwm", PwmID: 1},
			{Name: "12", DeviceName: "gpiochip0", LineNumber: 75},
			{Name: "16", DeviceName: "gpiochip0", LineNumber: 79},
			{Name: "18", DeviceName: "gpiochip0", LineNumber: 78},
			{Name: "22", DeviceName: "gpiochip0", LineNumber: 71},
			{Name: "24", DeviceName: "gpiochip0", LineNumber: 233},
			{Name: "26", DeviceName: "gpiochip0", LineNumber: 7},
		},
		Compats: []string{"xunlong,orangepi-zero2", "allwinner,sun50i-h616"},
	},
}

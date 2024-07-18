package orangepi

import "go.viam.com/rdk/components/board/genericlinux"

const (
	opzero2  = "OrangePi Zero2"
	op3lts   = "OrangePi 3 LTS"
	opzero2w = "OrangePi Zero 2W"
)

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	opzero2w: {
		// OP zero 2w user manual: https://drive.google.com/drive/folders/1KIZMMDBlqf1rKmOEhGH7_7A-COAgYoGZ
		// Gpio pins can be found on page 131.
		PinDefinitions: []genericlinux.PinDefinition{
			{Name: "3", DeviceName: "gpiochip0", LineNumber: 264, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", DeviceName: "gpiochip0", LineNumber: 263, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "7", DeviceName: "gpiochip0", LineNumber: 269, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "8", DeviceName: "gpiochip0", LineNumber: 224, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", DeviceName: "gpiochip0", LineNumber: 225, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", DeviceName: "gpiochip0", LineNumber: 226, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", DeviceName: "gpiochip0", LineNumber: 257, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", DeviceName: "gpiochip0", LineNumber: 227, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", DeviceName: "gpiochip0", LineNumber: 261, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", DeviceName: "gpiochip0", LineNumber: 270, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", DeviceName: "gpiochip0", LineNumber: 228, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", DeviceName: "gpiochip0", LineNumber: 231, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", DeviceName: "gpiochip0", LineNumber: 232, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", DeviceName: "gpiochip0", LineNumber: 262, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", DeviceName: "gpiochip0", LineNumber: 230, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", DeviceName: "gpiochip0", LineNumber: 229, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", DeviceName: "gpiochip0", LineNumber: 233, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "27", DeviceName: "gpiochip0", LineNumber: 266, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "28", DeviceName: "gpiochip0", LineNumber: 265, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "29", DeviceName: "gpiochip0", LineNumber: 256, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "31", DeviceName: "gpiochip0", LineNumber: 271, PwmChipSysfsDir: "", PwmID: -1},
			// When we can switch between gpio and pwm, this would have line number 267.
			{Name: "32", DeviceName: "gpiochip0", LineNumber: -1, PwmChipSysfsDir: "300a000.pwm", PwmID: 1},
			// When we can switch between gpio and pwm, this would have line number 268.
			{Name: "33", DeviceName: "gpiochip0", LineNumber: -1, PwmChipSysfsDir: "300a000.pwm", PwmID: 2},
			{Name: "35", DeviceName: "gpiochip0", LineNumber: 258, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "36", DeviceName: "gpiochip0", LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "37", DeviceName: "gpiochip0", LineNumber: 272, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "38", DeviceName: "gpiochip0", LineNumber: 260, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "40", DeviceName: "gpiochip0", LineNumber: 259, PwmChipSysfsDir: "", PwmID: -1},
		},
		Compats: []string{"xunlong,orangepi-zero2w", "allwinner,sun50i-h616"},
	},
	opzero2: {
		// OP zero 2 user manual: https://drive.google.com/drive/folders/1ToDjWZQptABxfiRwaeYW1WzQILM5iwpb
		// Gpio pins can be found on page 147.
		PinDefinitions: []genericlinux.PinDefinition{
			{Name: "3", DeviceName: "gpiochip0", LineNumber: 229, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", DeviceName: "gpiochip0", LineNumber: 228, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "7", DeviceName: "gpiochip0", LineNumber: 73, PwmChipSysfsDir: "", PwmID: -1},
			// Note that gpio input cannot be used on this pin.
			{Name: "8", DeviceName: "gpiochip0", LineNumber: 226, PwmChipSysfsDir: "300a000.pwm", PwmID: 2},
			// Note that gpio input cannot be used on this pin.
			{Name: "10", DeviceName: "gpiochip0", LineNumber: 227, PwmChipSysfsDir: "300a000.pwm", PwmID: 1},
			{Name: "11", DeviceName: "gpiochip0", LineNumber: 70, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", DeviceName: "gpiochip0", LineNumber: 75, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", DeviceName: "gpiochip0", LineNumber: 69, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", DeviceName: "gpiochip0", LineNumber: 72, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", DeviceName: "gpiochip0", LineNumber: 79, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", DeviceName: "gpiochip0", LineNumber: 78, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", DeviceName: "gpiochip0", LineNumber: 231, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", DeviceName: "gpiochip0", LineNumber: 232, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", DeviceName: "gpiochip0", LineNumber: 71, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", DeviceName: "gpiochip0", LineNumber: 233, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", DeviceName: "gpiochip0", LineNumber: 74, PwmChipSysfsDir: "", PwmID: -1},
		},
		Compats: []string{"xunlong,orangepi-zero2", "allwinner,sun50i-h616"},
	},
	op3lts: {
		// OP 3 LTS user manual: https://drive.google.com/file/d/1jka7avWnzNeTIQFkk78LoJdygWaGH2iu/view
		// Gpio pins can be found on page 145.
		PinDefinitions: []genericlinux.PinDefinition{
			{Name: "3", DeviceName: "gpiochip1", LineNumber: 122, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "5", DeviceName: "gpiochip1", LineNumber: 121, PwmChipSysfsDir: "", PwmID: -1},
			// Note that gpio input cannot be used on this pin.
			{Name: "7", DeviceName: "gpiochip1", LineNumber: 118, PwmChipSysfsDir: "300a000.pwm", PwmID: 0},
			{Name: "8", DeviceName: "gpiochip0", LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "10", DeviceName: "gpiochip0", LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "11", DeviceName: "gpiochip1", LineNumber: 120, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "12", DeviceName: "gpiochip1", LineNumber: 114, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "13", DeviceName: "gpiochip1", LineNumber: 119, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "15", DeviceName: "gpiochip0", LineNumber: 10, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "16", DeviceName: "gpiochip1", LineNumber: 111, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "18", DeviceName: "gpiochip1", LineNumber: 112, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "19", DeviceName: "gpiochip1", LineNumber: 229, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "21", DeviceName: "gpiochip1", LineNumber: 230, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "22", DeviceName: "gpiochip1", LineNumber: 117, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "23", DeviceName: "gpiochip1", LineNumber: 228, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "24", DeviceName: "gpiochip1", LineNumber: 227, PwmChipSysfsDir: "", PwmID: -1},
			{Name: "26", DeviceName: "gpiochip0", LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
		},
		Compats: []string{"xunlong,orangepi-3-lts", "allwinner,sun50i-h6"},
	},
}

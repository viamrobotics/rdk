package orangepi

import "go.viam.com/rdk/components/board/genericlinux"

const (
	opzero2 = "OrangePi Zero2"
	op3lts  = "OrangePi 3 LTS"
)

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	opzero2: {
		// OP zero 2 user manual: https://drive.google.com/drive/folders/1ToDjWZQptABxfiRwaeYW1WzQILM5iwpb
		// Gpio pins can be found on page 147.
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
	op3lts: {
		// OP 3 LTS user manual: https://drive.google.com/file/d/1jka7avWnzNeTIQFkk78LoJdygWaGH2iu/view
		// Gpio pins can be found on page 145.
		PinDefinitions: []genericlinux.PinDefinition{
			{Name: "3", DeviceName: "gpiochip1", LineNumber: 122},
			{Name: "5", DeviceName: "gpiochip1", LineNumber: 121},
			// When we can switch between gpio and pwm, the line number would be 118.
			{Name: "7", DeviceName: "gpiochip1", LineNumber: -1, PwmChipSysfsDir: "300a000.pwm", PwmID: 0},
			{Name: "8", DeviceName: "gpiochip0", LineNumber: 2},
			{Name: "10", DeviceName: "gpiochip0", LineNumber: 3},
			{Name: "11", DeviceName: "gpiochip1", LineNumber: 120},
			{Name: "12", DeviceName: "gpiochip1", LineNumber: 114},
			{Name: "13", DeviceName: "gpiochip1", LineNumber: 119},
			{Name: "15", DeviceName: "gpiochip0", LineNumber: 10},
			{Name: "16", DeviceName: "gpiochip1", LineNumber: 111},
			{Name: "18", DeviceName: "gpiochip1", LineNumber: 112},
			{Name: "19", DeviceName: "gpiochip1", LineNumber: 229},
			{Name: "21", DeviceName: "gpiochip1", LineNumber: 230},
			{Name: "22", DeviceName: "gpiochip1", LineNumber: 117},
			{Name: "23", DeviceName: "gpiochip1", LineNumber: 228},
			{Name: "24", DeviceName: "gpiochip1", LineNumber: 227},
			{Name: "26", DeviceName: "gpiochip0", LineNumber: 8},
		},
		Compats: []string{"xunlong,orangepi-3-lts", "allwinner,sun50i-h6"},
	},
}

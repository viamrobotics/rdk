package intel

import "go.viam.com/rdk/components/board/genericlinux"

const upboard = "up_4000"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	upboard: {
		[]genericlinux.PinDefinition{
			/*
				pinout for up4000: https://github.com/up-board/up-community/wiki/Pinout_UP4000

				GPIOChipRelativeIDs: {ngpio : base-ngpio}
				GPIOChipSysFSDir: path to the directory of a chip. Can be found from the output of gpiodetect

			*/
			// GPIO pin definition
			{map[int]int{78: 73}, map[int]string{}, "INT3452:01", 29, 0, "GPIO10", "", "", -1},
			{map[int]int{77: 46}, map[int]string{}, "INT3452:01", 31, 0, "BCM26", "", "", -1},
			{map[int]int{78: 37}, map[int]string{}, "INT3452:00", 16, 0, "BCM23", "", "", -1},
			{map[int]int{77: 48}, map[int]string{}, "INT3452:01", 18, 0, "BCM24", "", "", -1},
			{map[int]int{77: 45}, map[int]string{}, "INT3452:01", 22, 0, "BCM25", "", "", -1},

			{map[int]int{77: 46}, map[int]string{}, "INT3452:01", 37, 0, "BCM26", "", "", -1},
			{map[int]int{47: 17}, map[int]string{}, "INT3452:02", 35, 0, "BCM19", "", "", -1}, // PCM_FS

			{map[int]int{77: 75}, map[int]string{}, "INT3452:01", 13, 0, "BMC27", "", "", -1},

			// ttyS4 UART
			{map[int]int{78: 43}, map[int]string{}, "INT3452:00", 8, 0, "BCM14_TXD", "", "", -1},  // UART1_TX device: ttyS4
			{map[int]int{78: 42}, map[int]string{}, "INT3452:00", 10, 0, "BCM15_RXD", "", "", -1}, // UART1_RX device: ttyS4
			{map[int]int{78: 44}, map[int]string{}, "INT3452:00", 11, 0, "BCM17", "", "", -1},     // UART1_RTS device: ttyS4
			{map[int]int{78: 45}, map[int]string{}, "INT3452:00", 36, 0, "BMC16", "", "", -1},     // UART1_CTS device: ttyS4

			// I2c
			{map[int]int{78: 28}, map[int]string{}, "INT3452:00", 3, 0, "BCM2_SDA", "", "", -1},     // I2C1_SDA device: i2c-5
			{map[int]int{78: 29}, map[int]string{}, "INT3452:00", 5, 0, "BVM3_SCL", "", "", -1},     // I2C1_SCL device: i2c-5
			{map[int]int{78: 31}, map[int]string{}, "INT3452:00", 28, 0, "BMC1_ID_SCL", "", "", -1}, // ID_SC device: i2c-0

			// pwm
			{map[int]int{78: 35}, map[int]string{}, "INT3452:00", 33, 0, "BMC13_PWM1", "", "", 0}, // PWM1 device: pwmchip1/pwm0
			{map[int]int{78: 34}, map[int]string{}, "INT3452:00", 32, 0, "BCM12_PWM0", "", "", 0}, // PWM0 device: pwmchip0/pwm0

			{map[int]int{77: 76}, map[int]string{}, "INT3452:01", 7, 0, "BCM4", "", "", -1},        // ADC0 device: ii0:device0
			{map[int]int{77: 65}, map[int]string{}, "INT3452:01", 19, 0, "BCM10_MOSI", "", "", -1}, // SPI_MOSI
			{map[int]int{77: 64}, map[int]string{}, "INT3452:01", 21, 0, "BCM9_MISO", "", "", -1},  // SPI_MISO
			{map[int]int{77: 61}, map[int]string{}, "INT3452:01", 23, 0, "BCM11_SCLK", "", "", -1}, // SPI_CLK

			{map[int]int{78: 30}, map[int]string{}, "INT3452:00", 27, 0, "BCM0_ID_SD", "", "", -1}, // ID_SD

			{map[int]int{47: 16}, map[int]string{}, "INT3452:02", 12, 0, "BCM15_RXD", "", "", -1}, // PCM_CLK
			{map[int]int{77: 62}, map[int]string{}, "INT3452:01", 24, 0, "BCM8_CE0", "", "", -1},  // SPI_CS0 device: spidev2.0
			{map[int]int{77: 63}, map[int]string{}, "INT3452:01", 26, 0, "BCM7_CE1", "", "", -1},  // SPI_CS1 device: spidev2.1

			{map[int]int{47: 18}, map[int]string{}, "INT3452:02", 38, 0, "BCM20", "", "", -1}, // PCM_DIN
			{map[int]int{47: 19}, map[int]string{}, "INT3452:02", 40, 0, "BCM21", "", "", -1}, // PCM_DOUT
			{map[int]int{77: 74}, map[int]string{}, "INT3452:01", 15, 0, "BCM22", "", "", -1},
		},
		[]string{"UP-APL03"},
	},
}

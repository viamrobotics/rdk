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
			{map[int]int{28: 5}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 29, 0, "GPIO10", "", "", -1},
			{map[int]int{28: 6}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 31, 0, "GPIO11", "", "", -1},
			{map[int]int{28: 16}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 36, 0, "GPIO25", "", "", -1},
			{map[int]int{28: 23}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 16, 0, "GPIO18", "", "", -1},
			{map[int]int{28: 24}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 18, 0, "GPIO19", "", "", -1},
			{map[int]int{28: 25}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 22, 0, "GPIO20", "", "", -1},
			{map[int]int{28: 26}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 37, 0, "GPIO14", "", "", -1},
			{map[int]int{28: 26}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 37, 0, "GPIO403", "", "", -1},
			{map[int]int{28: 27}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 13, 0, "GPIO4", "", "", -1},

			{map[int]int{28: 14}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 8, 0, "GPIO15", "", "", -1},
			{map[int]int{28: 15}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 10, 0, "GPIO16", "", "", -1},
			{map[int]int{28: 17}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 11, 0, "GPIO3", "", "", -1},
			{map[int]int{28: 16}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 36, 0, "GPIO25", "", "", -1},

			{map[int]int{28: 2}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 3, 0, "GPIO0", "", "", -1},
			{map[int]int{28: 3}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 5, 0, "GPIO1", "", "", -1},
			{map[int]int{28: 1}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 28, 0, "GPIO23", "", "", -1},

			{map[int]int{28: 13}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 33, 0, "GPIO12", "", "", 1},
			{map[int]int{28: 12}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 32, 0, "GPIO24", "", "", 1},

			{map[int]int{28: 4}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 7, 0, "GPIO2", "", "", -1},
			{map[int]int{28: 10}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 19, 0, "GPIO6", "", "", -1},
			{map[int]int{28: 9}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 21, 0, "GPIO7", "", "", -1},
			{map[int]int{28: 11}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 23, 0, "GPIO8", "", "", -1},

			{map[int]int{28: 0}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 27, 0, "GPIO9", "", "", -1},
			{map[int]int{28: 19, 78: 327}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 35, 0, "GPIO13", "", "", -1},

			{map[int]int{28: 18}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 12, 0, "GPIO17", "", "", -1},
			{map[int]int{28: 8}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 24, 0, "GPIO21", "", "", -1},
			{map[int]int{28: 7}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 26, 0, "GPIO22", "", "", -1},

			{map[int]int{28: 20}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 38, 0, "GPIO26", "", "", -1},
			{map[int]int{28: 21}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 40, 0, "GPIO27", "", "", -1},
			{map[int]int{28: 22}, map[int]string{}, "AANT0F04:00/upboard-pinctrl.0", 15, 0, "GPIO5", "", "", -1},
		},
		[]string{"UP-APL03"},
	},
}

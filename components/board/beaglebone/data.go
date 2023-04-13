package beaglebone

import "go.viam.com/rdk/components/board/genericlinux"

const bbAi = "bb_Ai64"

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	bbAi: {
		PinDefinitions: []genericlinux.PinDefinition{
			// GPIOChipRelativeIDs {NGPIO: LINENUM} -> {128: 93}
			// PinNumberBoard {914} -> PinNameCVM3 "P9_14"
			// Duplicate pins are commented out of the

			// ******** DATA MAPPING ********************************
			// PWMs contain a number other than -1 in the last element of map
			// beaglebone pwm mapping sys/devices/platform/bus@100000/*.pwm

			{map[int]int{128: 89}, map[int]string{}, "600000.gpio", 813, 0, "P8_13", "", "3000000.pwm", 1}, // pwmchip0 V27 EHRPWM0_A
			{map[int]int{128: 88}, map[int]string{}, "600000.gpio", 819, 0, "P8_19", "", "3000000.pwm", 0}, // pwmchip0 V29 EHRPWM0_B
			{map[int]int{128: 93}, map[int]string{}, "600000.gpio", 914, 0, "P9_14", "", "3020000.pwm", 0}, // pwmchip4 U27 EHRPWM2_A
			{map[int]int{128: 94}, map[int]string{}, "600000.gpio", 916, 0, "P9_16", "", "3020000.pwm", 1}, // pwmchip4 U24 EHRPWM2_B
			{map[int]int{128: 39}, map[int]string{}, "600000.gpio", 921, 0, "P9_21", "", "3010000.pwm", 0}, // pwmchip2
			{map[int]int{128: 38}, map[int]string{}, "600000.gpio", 922, 0, "P9_22", "", "3010000.pwm", 1}, // pwmchip2 BOOTMODE1

			// Timer only PWM
			{map[int]int{128: 26}, map[int]string{}, "600000.gpio", 832, 0, "P8_32", "", "", -1},
			{map[int]int{128: 25}, map[int]string{}, "600000.gpio", 833, 0, "P8_33", "", "", -1},
			{map[int]int{128: 24}, map[int]string{}, "600000.gpio", 835, 0, "P8_35", "", "", -1},
			{map[int]int{128: 11}, map[int]string{}, "600000.gpio", 837, 0, "P8_37", "", "", -1},
			{map[int]int{128: 46}, map[int]string{}, "600000.gpio", 927, 0, "P9_27", "", "", -1},
			{map[int]int{36: 14}, map[int]string{}, "601000.gpio", 929, 0, "9_29", "", "", -1},
			{map[int]int{36: 13}, map[int]string{}, "601000.gpio", 930, 0, "P9_30", "", "", -1},
			{map[int]int{128: 123}, map[int]string{}, "600000.gpio", 942, 0, "P9_42", "", "", -1},

			// GPIO only pins
			// beaglebone gpio mapping uses directory sys/devices/platform/bus@100000/*.gpio
			// beaglebone 600000.gpio/ (128 lines) corresponds to gpiochip1 and /sys/class/gpio/gpiochip300/
			// beaglebone 601000.gpio/ (36 lines) corresponds to gpiochip2 and /sys/class/gpio/gpiochip264/
			{map[int]int{128: 14}, map[int]string{}, "600000.gpio", 808, 0, "P8_08", "", "", -1},
			{map[int]int{128: 15}, map[int]string{}, "600000.gpio", 807, 0, "P8_07", "", "", -1},
			{map[int]int{128: 16}, map[int]string{}, "600000.gpio", 810, 0, "P8_10", "", "", -1},
			{map[int]int{128: 17}, map[int]string{}, "600000.gpio", 809, 0, "P8_09", "", "", -1},
			{map[int]int{128: 1}, map[int]string{}, "600000.gpio", 911, 0, "P9_11", "", "", -1},
			{map[int]int{128: 2}, map[int]string{}, "600000.gpio", 913, 0, "P9_13", "", "", -1},
			{map[int]int{128: 3}, map[int]string{}, "600000.gpio", 817, 0, "P8_17", "", "", -1},
			{map[int]int{128: 4}, map[int]string{}, "600000.gpio", 818, 0, "P8_18", "", "", -1},
			{map[int]int{128: 5}, map[int]string{}, "600000.gpio", 822, 0, "P8_22", "", "", -1},
			{map[int]int{128: 6}, map[int]string{}, "600000.gpio", 824, 0, "P8_24", "", "", -1},
			{map[int]int{128: 7}, map[int]string{}, "600000.gpio", 834, 0, "P8_34", "", "", -1},
			{map[int]int{128: 8}, map[int]string{}, "600000.gpio", 836, 0, "P8_36", "", "", -1},
			{map[int]int{128: 9}, map[int]string{}, "600000.gpio", 838, 0, "P8_38", "", "", -1},
			{map[int]int{128: 10}, map[int]string{}, "600000.gpio", 923, 0, "P9_23", "", "", -1},
			{map[int]int{128: 12}, map[int]string{}, "600000.gpio", 926, 0, "P9_26", "", "", -1},
			{map[int]int{128: 20}, map[int]string{}, "600000.gpio", 803, 0, "P8_03", "", "", -1},
			{map[int]int{128: 28}, map[int]string{}, "600000.gpio", 917, 0, "P9_17", "", "", -1},
			{map[int]int{128: 30}, map[int]string{}, "600000.gpio", 821, 0, "P8_21", "", "", -1},
			{map[int]int{128: 31}, map[int]string{}, "600000.gpio", 823, 0, "P8_23", "", "", -1},
			{map[int]int{128: 32}, map[int]string{}, "600000.gpio", 831, 0, "P8_31", "", "", -1},
			{map[int]int{128: 33}, map[int]string{}, "600000.gpio", 805, 0, "P8_05", "", "", -1},
			{map[int]int{128: 34}, map[int]string{}, "600000.gpio", 806, 0, "P8_06", "", "", -1},
			{map[int]int{128: 35}, map[int]string{}, "600000.gpio", 825, 0, "P8_25", "", "", -1},
			{map[int]int{128: 40}, map[int]string{}, "600000.gpio", 918, 0, "P9_18", "", "", -1},
			{map[int]int{128: 43}, map[int]string{}, "600000.gpio", 928, 0, "P9_28", "", "", -1},
			{map[int]int{128: 45}, map[int]string{}, "600000.gpio", 912, 0, "P9_12", "", "", -1},
			{map[int]int{128: 47}, map[int]string{}, "600000.gpio", 915, 0, "P9_15", "", "", -1},
			{map[int]int{128: 48}, map[int]string{}, "600000.gpio", 804, 0, "P8_04", "", "", -1}, // BOOTMODE2
			{map[int]int{128: 50}, map[int]string{}, "600000.gpio", 933, 0, "P9_33B", "", "", -1},
			{map[int]int{128: 51}, map[int]string{}, "600000.gpio", 826, 0, "P8_26", "", "", -1},
			{map[int]int{128: 52}, map[int]string{}, "600000.gpio", 931, 0, "P9_31", "", "", -1},
			{map[int]int{128: 55}, map[int]string{}, "600000.gpio", 935, 0, "P9_35", "", "", -1},
			{map[int]int{128: 56}, map[int]string{}, "600000.gpio", 936, 0, "P9_36", "", "", -1},
			{map[int]int{128: 57}, map[int]string{}, "600000.gpio", 937, 0, "P9_37", "", "", -1},
			{map[int]int{128: 58}, map[int]string{}, "600000.gpio", 938, 0, "P9_38", "", "", -1},
			{map[int]int{128: 59}, map[int]string{}, "600000.gpio", 812, 0, "P8_12", "", "", -1},
			{map[int]int{128: 60}, map[int]string{}, "600000.gpio", 811, 0, "P8_11", "", "", -1}, // BOOTMODE7
			{map[int]int{128: 61}, map[int]string{}, "600000.gpio", 815, 0, "P8_15", "", "", -1},
			{map[int]int{128: 62}, map[int]string{}, "600000.gpio", 816, 0, "P8_16", "", "", -1},
			{map[int]int{128: 65}, map[int]string{}, "600000.gpio", 843, 0, "P8_43", "", "", -1},
			{map[int]int{128: 66}, map[int]string{}, "600000.gpio", 844, 0, "P8_44", "", "", -1},
			{map[int]int{128: 67}, map[int]string{}, "600000.gpio", 841, 0, "P8_41", "", "", -1},
			{map[int]int{128: 68}, map[int]string{}, "600000.gpio", 842, 0, "P8_42", "", "", -1}, // BOOTMODE6
			{map[int]int{128: 69}, map[int]string{}, "600000.gpio", 839, 0, "P8_39", "", "", -1},
			{map[int]int{128: 70}, map[int]string{}, "600000.gpio", 840, 0, "P8_40", "", "", -1},
			{map[int]int{128: 71}, map[int]string{}, "600000.gpio", 827, 0, "P8_27", "", "", -1},
			{map[int]int{128: 72}, map[int]string{}, "600000.gpio", 828, 0, "P8_28", "", "", -1},
			{map[int]int{128: 73}, map[int]string{}, "600000.gpio", 829, 0, "P8_29", "", "", -1},
			{map[int]int{128: 74}, map[int]string{}, "600000.gpio", 830, 0, "P8_30", "", "", -1},
			{map[int]int{128: 75}, map[int]string{}, "600000.gpio", 814, 0, "P8_14", "", "", -1},
			{map[int]int{128: 76}, map[int]string{}, "600000.gpio", 820, 0, "P8_20", "", "", -1},
			{map[int]int{128: 78}, map[int]string{}, "600000.gpio", 919, 0, "P9_19", "", "", -1},
			{map[int]int{128: 79}, map[int]string{}, "600000.gpio", 845, 0, "P8_45", "", "", -1},
			{map[int]int{128: 80}, map[int]string{}, "600000.gpio", 846, 0, "P8_46", "", "", -1}, // BOOTMODE3
			{map[int]int{128: 119}, map[int]string{}, "600000.gpio", 924, 0, "P9_24", "", "", -1},
			{map[int]int{128: 127}, map[int]string{}, "600000.gpio", 925, 0, "P9_25", "", "", -1},
			{map[int]int{36: 0}, map[int]string{}, "601000.gpio", 0, 0, "P9_41", "", "", -1},
			{map[int]int{36: 2}, map[int]string{}, "601000.gpio", 0, 0, "P9_20", "", "", -1},
		},
		Compats: []string{"beagle,j721e-beagleboneai64", "ti,j721e"},
	},
}

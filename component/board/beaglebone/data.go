package bb

import "go.viam.com/rdk/component/board/commonsysfs"

const bbAi = "bb_Ai64"

var boardInfoMappings = map[string]commonsysfs.BoardInformation{
	bbAi: {
		PinDefinitions: []commonsysfs.PinDefinition{
			// GPIOChipRelativeIDs: map[int]int{NGPIO: LINENUM},
			// GPIONames: map[int]string{emptyforbeagle} ,
			// GPIOChipSysFSDir: "600000.gpio"/"601000.gpio",
			// PinNumberBoard: 3, PinNumberBCM: 0-emptyfor beagle,
			// PinNameCVM: "P9_11", can be anything, used as keys for our board mappings.
			// PinNameTegraSOC: "" - emptyforbeagle,
			// PWMChipSysFSDir: "3000000.pwm"/"3010000.pwm"/"3020000.pwm"/"3030000.pwm"/"304000.pwm",
			// PWMID: -1 for nopwm, 1 for yes, 0 for yes and matching line to 1},

			// ******** GPIO CHIPS *********************************
			// From /sys/class/gpio/
			// cat gpiochip300/label -> 600000.gpio : /sys/devices/platform/bus@100000/600000.gpio/ contains gpiochip 1
			// cat gpiochip264/label -> 601000.gpio	: /sys/devices/platform/bus@100000/601000.gpio/ contains gpiochip 2

			// ******** PWM CHIPS **********************************
			// output of /sys/class/pwm$ ls -ls
			// 0 lrwxrwxrwx 1 root root 0 Aug 17 15:50 pwmchip0 -> ../../devices/platform/bus@100000/3000000.pwm/pwm/pwmchip0
			// 0 lrwxrwxrwx 1 root root 0 Aug 17 15:50 pwmchip10 -> ../../devices/platform/bus@100000/3050000.pwm/pwm/pwmchip10
			// 0 lrwxrwxrwx 1 root root 0 Aug 17 15:50 pwmchip2 -> ../../devices/platform/bus@100000/3010000.pwm/pwm/pwmchip2
			// 0 lrwxrwxrwx 1 root root 0 Aug 17 15:50 pwmchip4 -> ../../devices/platform/bus@100000/3020000.pwm/pwm/pwmchip4
			// 0 lrwxrwxrwx 1 root root 0 Aug 17 15:50 pwmchip6 -> ../../devices/platform/bus@100000/3030000.pwm/pwm/pwmchip6
			// 0 lrwxrwxrwx 1 root root 0 Aug 17 15:50 pwmchip8 -> ../../devices/platform/bus@100000/3040000.pwm/pwm/pwmchip8

			// ******** DATA MAPPING ********************************
			// PWMs
			{map[int]int{128: 89}, map[int]string{}, "600000.gpio", 813, 0, "P8_13", "", "3000000.pwm ", 0}, // pwmchip0 V27 EHRPWM0_A
			{map[int]int{128: 88}, map[int]string{}, "600000.gpio", 819, 0, "P8_19", "", "3000000.pwm ", 1}, // pwmchip0 V29 EHRPWM0_B
			{map[int]int{128: 93}, map[int]string{}, "600000.gpio", 914, 0, "P9_14", "", "3030000.pwm", 1},  // pwmchip4 U27 EHRPWM2_A
			{map[int]int{128: 94}, map[int]string{}, "600000.gpio", 916, 0, "P9_16", "", "3030000.pwm", 0},  // pwmchip4 U24 EHRPWM2_B
			{map[int]int{128: 39}, map[int]string{}, "600000.gpio", 921, 0, "P9_21", "", "3020000.pwm", 1},  // pwmchip2
			{map[int]int{128: 38}, map[int]string{}, "600000.gpio", 922, 0, "P9_22", "", "3020000.pwm", 0},  // pwmchip2 BOOTMODE1
			// {map[int]int{128: 90}, map[int]string{}, "600000.gpio", 921, 0, "P9_21B", "", "3020000.pwm", 1},
			// {map[int]int{128: 91}, map[int]string{}, "600000.gpio", 922, 0, "P9_22B", "", "3020000.pwm", 0},

			// Timer only PWM
			{map[int]int{128: 26}, map[int]string{}, "600000.gpio", 832, 0, "P8_32", "", "", -1},
			{map[int]int{128: 25}, map[int]string{}, "600000.gpio", 833, 0, "P8_33", "", "", -1},
			// {map[int]int{128: 111}, map[int]string{}, "600000.gpio", 833, 0, "P8_33B", "", "", -1},
			{map[int]int{128: 24}, map[int]string{}, "600000.gpio", 835, 0, "P8_35", "", "", -1},
			// {map[int]int{128: 116}, map[int]string{}, "600000.gpio", 835, 0, "P8_35B", "", "", -1},
			{map[int]int{128: 11}, map[int]string{}, "600000.gpio", 837, 0, "P8_37", "", "", -1},
			// {map[int]int{128: 11}, map[int]string{}, "600000.gpio", 837, 0, "P8_37B", "", "", -1},
			{map[int]int{128: 46}, map[int]string{}, "600000.gpio", 927, 0, "P9_27", "", "", -1},
			// {map[int]int{128: 124}, map[int]string{}, "600000.gpio", 927, 0, "P9_27B", "", "", -1},
			{map[int]int{36: 14}, map[int]string{}, "601000.gpio", 929, 0, "9_29", "", "", -1},
			// {map[int]int{128: 53}, map[int]string{}, "600000.gpio", 929, 0, "P9_29B", "", "", -1},
			{map[int]int{36: 13}, map[int]string{}, "601000.gpio", 930, 0, "P9_30", "", "", -1},
			// {map[int]int{128: 44}, map[int]string{}, "600000.gpio", 930, 0, "P9_30B", "", "", -1},
			{map[int]int{128: 123}, map[int]string{}, "600000.gpio", 942, 0, "P9_42", "", "", -1},
			// {map[int]int{128: 18}, map[int]string{}, "600000.gpio", 942, 0, "P9_42B", "", "", -1},

			// GPIO only pins, secondary pins commented out
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
			// {map[int]int{128: 105}, map[int]string{}, "600000.gpio", 838, 0, "P8_38B", "", "", -1},
			{map[int]int{128: 10}, map[int]string{}, "600000.gpio", 923, 0, "P9_23", "", "", -1},
			{map[int]int{128: 12}, map[int]string{}, "600000.gpio", 926, 0, "P9_26", "", "", -1},
			// {map[int]int{128: 118}, map[int]string{}, "600000.gpio", 926, 0, "P9_26A", "", "", -1},
			{map[int]int{128: 20}, map[int]string{}, "600000.gpio", 803, 0, "P8_03", "", "", -1},
			{map[int]int{128: 28}, map[int]string{}, "600000.gpio", 917, 0, "P9_17", "", "", -1},
			// {map[int]int{128: 115}, map[int]string{}, "600000.gpio", 917, 0, "P9_17B", "", "", -1},
			{map[int]int{128: 30}, map[int]string{}, "600000.gpio", 821, 0, "P8_21", "", "", -1},
			{map[int]int{128: 31}, map[int]string{}, "600000.gpio", 823, 0, "P8_23", "", "", -1},
			{map[int]int{128: 32}, map[int]string{}, "600000.gpio", 831, 0, "P8_31", "", "", -1},
			{map[int]int{128: 33}, map[int]string{}, "600000.gpio", 805, 0, "P8_05", "", "", -1},
			{map[int]int{128: 34}, map[int]string{}, "600000.gpio", 806, 0, "P8_06", "", "", -1},
			{map[int]int{128: 35}, map[int]string{}, "600000.gpio", 825, 0, "P8_25", "", "", -1},
			{map[int]int{128: 40}, map[int]string{}, "600000.gpio", 918, 0, "P9_18", "", "", -1},
			// {map[int]int{128: 120}, map[int]string{}, "600000.gpio", 918, 0, "P9_18B", "", "", -1},
			{map[int]int{128: 43}, map[int]string{}, "600000.gpio", 928, 0, "P9_28", "", "", -1},
			// {map[int]int{36: 11}, map[int]string{}, "601000.gpio", 0, 0, "P9_28A", "", "", -1},
			{map[int]int{128: 45}, map[int]string{}, "600000.gpio", 912, 0, "P9_12", "", "", -1},
			{map[int]int{128: 47}, map[int]string{}, "600000.gpio", 915, 0, "P9_15", "", "", -1},
			{map[int]int{128: 48}, map[int]string{}, "600000.gpio", 804, 0, "P8_04", "", "", -1}, // BOOTMODE2
			{map[int]int{128: 50}, map[int]string{}, "600000.gpio", 933, 0, "P9_33B", "", "", -1},
			{map[int]int{128: 51}, map[int]string{}, "600000.gpio", 826, 0, "P8_26", "", "", -1},
			{map[int]int{128: 52}, map[int]string{}, "600000.gpio", 931, 0, "P9_31", "", "", -1},
			// {map[int]int{36: 12}, map[int]string{}, "601000.gpio", 0, 0, "P9_31A", "", "", -1},
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
			// {map[int]int{128: 54}, map[int]string{}, "600000.gpio", 939, 0, "P9_39B", "", "", -1},
			{map[int]int{128: 70}, map[int]string{}, "600000.gpio", 840, 0, "P8_40", "", "", -1},
			// {map[int]int{128: 81}, map[int]string{}, "600000.gpio", 940, 0, "P9_40B", "", "", -1},
			{map[int]int{128: 71}, map[int]string{}, "600000.gpio", 827, 0, "P8_27", "", "", -1},
			{map[int]int{128: 72}, map[int]string{}, "600000.gpio", 828, 0, "P8_28", "", "", -1},
			{map[int]int{128: 73}, map[int]string{}, "600000.gpio", 829, 0, "P8_29", "", "", -1},
			{map[int]int{128: 74}, map[int]string{}, "600000.gpio", 830, 0, "P8_30", "", "", -1},
			{map[int]int{128: 75}, map[int]string{}, "600000.gpio", 814, 0, "P8_14", "", "", -1},
			{map[int]int{128: 76}, map[int]string{}, "600000.gpio", 820, 0, "P8_20", "", "", -1},
			{map[int]int{128: 78}, map[int]string{}, "600000.gpio", 919, 0, "P9_19", "", "", -1},
			// {map[int]int{36: 1}, map[int]string{}, "601000.gpio", 0, 0, "P9_19A", "", "", -1},
			{map[int]int{128: 79}, map[int]string{}, "600000.gpio", 845, 0, "P8_45", "", "", -1},
			{map[int]int{128: 80}, map[int]string{}, "600000.gpio", 846, 0, "P8_46", "", "", -1}, // BOOTMODE3
			{map[int]int{128: 119}, map[int]string{}, "600000.gpio", 924, 0, "P9_24", "", "", -1},
			// {map[int]int{128: 13}, map[int]string{}, "600000.gpio", 924, 0, "P9_24B", "", "", -1},
			{map[int]int{128: 127}, map[int]string{}, "600000.gpio", 925, 0, "P9_25", "", "", -1},
			// {map[int]int{128: 104}, map[int]string{}, "600000.gpio", 925, 0, "P9_25B", "", "", -1},
			/// 601000.gpio dir 36 lines
			{map[int]int{36: 0}, map[int]string{}, "601000.gpio", 0, 0, "P9_41", "", "", -1},
			{map[int]int{36: 2}, map[int]string{}, "601000.gpio", 0, 0, "P9_20", "", "", -1},
			// {map[int]int{128: 77}, map[int]string{}, "600000.gpio", 920, 0, "P9_20B", "", "", -1},
		},
		Compats: []string{"beagle,j721e-beagleboneai64ti,j721e"},
	},
}

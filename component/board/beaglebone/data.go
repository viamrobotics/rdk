package bb

import "go.viam.com/rdk/component/board/commonsysfs"

const bbAi = "bb_Ai64"

var boardInfoMappings = map[string]commonsysfs.BoardInformation{
	bbAi: {
		PinDefinitions: []commonsysfs.PinDefinition{
			// {GPIOChipRelativeIDs: map[int]int{128: 1}, GPIONames: map[int]string{128:"P."}, GPIOChipSysFSDir: "600000.gpio", PinNumberBoard: 0, PinNumberBCM: 0, PinNameCVM: "P9_11", PinNameTegraSOC: "", PWMChipSysFSDir: "", PWMID: -1 for no},
			// 600000.gpio dir 128 lines

			// TODO: Which pwm dirs are which:
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip0/device -> ../../../3000000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip10/device -> ../../../3050000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip2/device -> ../../../3010000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip4/device -> ../../../3020000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip6/device -> ../../../3030000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip8/device -> ../../../3040000.pwm
			{map[int]int{128: 1}, map[int]string{128: "P9.11"}, "600000.gpio", 911, 0, "P9_11", "", "", -1},
			{map[int]int{128: 2}, map[int]string{128: "P9.13"}, "600000.gpio", 913, 0, "P9_13", "", "", -1},
			{map[int]int{128: 3}, map[int]string{128: "P8.17"}, "600000.gpio", 817, 0, "P8_17", "", "", -1},
			{map[int]int{128: 4}, map[int]string{128: "P8.18"}, "600000.gpio", 818, 0, "P8_18", "", "", -1},
			{map[int]int{128: 5}, map[int]string{128: "P8.22"}, "600000.gpio", 822, 0, "P8_22", "", "", -1},
			{map[int]int{128: 6}, map[int]string{128: "P8.24"}, "600000.gpio", 824, 0, "P8_24", "", "", -1},
			{map[int]int{128: 7}, map[int]string{128: "P8.34"}, "600000.gpio", 834, 0, "P8_34", "", "", -1},
			{map[int]int{128: 8}, map[int]string{128: "P8.36"}, "600000.gpio", 836, 0, "P8_36", "", "", -1},
			{map[int]int{128: 9}, map[int]string{128: "P8.38A"}, "600000.gpio", 838, 0, "P8_38A", "", "", -1},
			{map[int]int{128: 10}, map[int]string{128: "P9.23"}, "600000.gpio", 923, 0, "P9_23", "", "", -1},
			{map[int]int{128: 11}, map[int]string{128: "P8.37B"}, "600000.gpio", 837, 0, "P8_37B", "", "", -1},
			{map[int]int{128: 12}, map[int]string{128: "P9.26B"}, "600000.gpio", 926, 0, "P9_26B", "", "", -1},
			{map[int]int{128: 13}, map[int]string{128: "P9.24B"}, "600000.gpio", 924, 0, "P9_24B", "", "", -1},
			{map[int]int{128: 14}, map[int]string{128: "P8.8"}, "600000.gpio", 808, 0, "P8_08", "", "", 1},  // pwm?
			{map[int]int{128: 15}, map[int]string{128: "P8.7"}, "600000.gpio", 807, 0, "P8_07", "", "", 1},  // pwm?
			{map[int]int{128: 16}, map[int]string{128: "P8.10"}, "600000.gpio", 810, 0, "P8_10", "", "", 1}, // pwm?
			{map[int]int{128: 17}, map[int]string{128: "P8.9"}, "600000.gpio", 809, 0, "P8_09", "", "", 1},  // pwm?
			{map[int]int{128: 18}, map[int]string{128: "P9.42B"}, "600000.gpio", 942, 0, "P9_42B", "", "", -1},
			{map[int]int{128: 20}, map[int]string{128: "P8.3"}, "600000.gpio", 803, 0, "P8_03", "", "", -1},
			{map[int]int{128: 24}, map[int]string{128: "P8.35A"}, "600000.gpio", 835, 0, "P8_35A", "", "", -1},
			{map[int]int{128: 25}, map[int]string{128: "P8.33A"}, "600000.gpio", 833, 0, "P8_33A", "", "", -1},
			{map[int]int{128: 26}, map[int]string{128: "P8.32A"}, "600000.gpio", 832, 0, "P8_32A", "", "", -1},
			{map[int]int{128: 28}, map[int]string{128: "P9.17A"}, "600000.gpio", 917, 0, "P9_17A", "", "", -1},
			{map[int]int{128: 30}, map[int]string{128: "P8.21"}, "600000.gpio", 821, 0, "P8_21", "", "", -1},
			{map[int]int{128: 31}, map[int]string{128: "P8.23"}, "600000.gpio", 823, 0, "P8_23", "", "", -1},
			{map[int]int{128: 32}, map[int]string{128: "P8.31A"}, "600000.gpio", 831, 0, "P8_31A", "", "", -1},
			{map[int]int{128: 33}, map[int]string{128: "P8.5"}, "600000.gpio", 805, 0, "P8_05", "", "", -1},
			{map[int]int{128: 34}, map[int]string{128: "P8.6"}, "600000.gpio", 806, 0, "P8_06", "", "", -1},
			{map[int]int{128: 35}, map[int]string{128: "P8.25"}, "600000.gpio", 825, 0, "P8_25", "", "", -1},
			{map[int]int{128: 38}, map[int]string{128: "P9.22A"}, "600000.gpio", 922, 0, "P9_22A (BOOTMODE1)", "", "", -1},
			{map[int]int{128: 39}, map[int]string{128: "P9.21A"}, "600000.gpio", 921, 0, "P9_21A", "", "", -1},
			{map[int]int{128: 40}, map[int]string{128: "P9.18A"}, "600000.gpio", 918, 0, "P9_18A", "", "", -1},
			{map[int]int{128: 43}, map[int]string{128: "P9.28B"}, "600000.gpio", 928, 0, "P9_28B", "", "", -1},
			{map[int]int{128: 44}, map[int]string{128: "P9.30B"}, "600000.gpio", 930, 0, "P9_30B", "", "", -1},
			{map[int]int{128: 45}, map[int]string{128: "P9.12"}, "600000.gpio", 912, 0, "P9_12", "", "", -1},
			{map[int]int{128: 46}, map[int]string{128: "P9.27A"}, "600000.gpio", 927, 0, "P9_27A", "", "", -1},
			{map[int]int{128: 47}, map[int]string{128: "P9.15"}, "600000.gpio", 915, 0, "P9_15", "", "", -1},
			{map[int]int{128: 48}, map[int]string{128: "P8.4"}, "600000.gpio", 804, 0, "P8_04 (BOOTMODE2)", "", "", -1},
			{map[int]int{128: 50}, map[int]string{128: "P9.33B"}, "600000.gpio", 933, 0, "P9_33B", "", "", -1},
			{map[int]int{128: 51}, map[int]string{128: "P8.26"}, "600000.gpio", 826, 0, "P8_26", "", "", -1},
			{map[int]int{128: 52}, map[int]string{128: "P9.31B"}, "600000.gpio", 931, 0, "P9_31B", "", "", -1},
			{map[int]int{128: 53}, map[int]string{128: "P9.29B"}, "600000.gpio", 929, 0, "P9_29B", "", "", -1},
			{map[int]int{128: 54}, map[int]string{128: "P9.39B"}, "600000.gpio", 939, 0, "P9_39B", "", "", -1},
			{map[int]int{128: 55}, map[int]string{128: "P9.35B"}, "600000.gpio", 935, 0, "P9_35B", "", "", -1},
			{map[int]int{128: 56}, map[int]string{128: "P9.36B"}, "600000.gpio", 936, 0, "P9_36B", "", "", -1},
			{map[int]int{128: 57}, map[int]string{128: "P9.37B"}, "600000.gpio", 937, 0, "P9_37B", "", "", -1},
			{map[int]int{128: 58}, map[int]string{128: "P9.38B"}, "600000.gpio", 938, 0, "P9_38B", "", "", -1},
			{map[int]int{128: 59}, map[int]string{128: "P8.12"}, "600000.gpio", 812, 0, "P8_12", "", "", -1},
			{map[int]int{128: 60}, map[int]string{128: "P8.11"}, "600000.gpio", 811, 0, "P8_11 (BOOTMODE7)", "", "", -1},
			{map[int]int{128: 61}, map[int]string{128: "P8.15"}, "600000.gpio", 815, 0, "P8_15", "", "", -1},
			{map[int]int{128: 62}, map[int]string{128: "P8.16"}, "600000.gpio", 816, 0, "P8_16", "", "", -1},
			{map[int]int{128: 65}, map[int]string{128: "P8.43"}, "600000.gpio", 843, 0, "P8_43", "", "", -1},
			{map[int]int{128: 66}, map[int]string{128: "P8.44"}, "600000.gpio", 844, 0, "P8_44", "", "", -1},
			{map[int]int{128: 67}, map[int]string{128: "P8.42"}, "600000.gpio", 841, 0, "P8_41", "", "", -1},
			{map[int]int{128: 68}, map[int]string{128: "P8.42"}, "600000.gpio", 842, 0, "P8_42 (BOOTMODE6)", "", "", -1},
			{map[int]int{128: 69}, map[int]string{128: "P8.39"}, "600000.gpio", 839, 0, "P8_39", "", "", -1},
			{map[int]int{128: 70}, map[int]string{128: "P8.40"}, "600000.gpio", 840, 0, "P8_40", "", "", -1},
			{map[int]int{128: 71}, map[int]string{128: "P8.27"}, "600000.gpio", 827, 0, "P8_27", "", "", -1},
			{map[int]int{128: 72}, map[int]string{128: "P8.28"}, "600000.gpio", 828, 0, "P8_28", "", "", -1},
			{map[int]int{128: 73}, map[int]string{128: "P8.29"}, "600000.gpio", 829, 0, "P8_29", "", "", -1},
			{map[int]int{128: 74}, map[int]string{128: "P8.30"}, "600000.gpio", 830, 0, "P8_30", "", "", -1},
			{map[int]int{128: 75}, map[int]string{128: "P8.14"}, "600000.gpio", 814, 0, "P8_14", "", "", -1},
			{map[int]int{128: 76}, map[int]string{128: "P8.20"}, "600000.gpio", 820, 0, "P8_20", "", "", -1},
			{map[int]int{128: 77}, map[int]string{128: "P9.20B"}, "600000.gpio", 920, 0, "P9_20B", "", "", -1},
			{map[int]int{128: 78}, map[int]string{128: "P9.29B"}, "600000.gpio", 919, 0, "P9_19B", "", "", -1},
			{map[int]int{128: 79}, map[int]string{128: "P8.45"}, "600000.gpio", 845, 0, "P8_45", "", "", -1},
			{map[int]int{128: 80}, map[int]string{128: "P8.46"}, "600000.gpio", 846, 0, "P8_46 (BOOTMODE3)", "", "", -1},
			{map[int]int{128: 81}, map[int]string{128: "P9.40B"}, "600000.gpio", 940, 0, "P9_40B", "", "", -1},
			{map[int]int{128: 88}, map[int]string{128: "P8.19"}, "600000.gpio", 819, 0, "P8_19", "", "", -1},
			{map[int]int{128: 89}, map[int]string{128: "P8.13"}, "600000.gpio", 813, 0, "P8_13", "", "", -1},
			{map[int]int{128: 90}, map[int]string{128: "P9.21B"}, "600000.gpio", 921, 0, "P9_21B", "", "", -1},
			{map[int]int{128: 91}, map[int]string{128: "P9.22B"}, "600000.gpio", 922, 0, "P9_22B", "", "", -1},
			{map[int]int{128: 93}, map[int]string{128: "P9.14"}, "600000.gpio", 914, 0, "P9_14", "", "", -1},
			{map[int]int{128: 94}, map[int]string{128: "P9.16"}, "600000.gpio", 916, 0, "P9_16", "", "", -1},
			{map[int]int{128: 104}, map[int]string{128: "P9.25B"}, "600000.gpio", 925, 0, "P9_25B", "", "", -1},
			{map[int]int{128: 105}, map[int]string{128: "P8.38B"}, "600000.gpio", 838, 0, "P8_38B", "", "", -1},
			{map[int]int{128: 106}, map[int]string{128: "P8.37A"}, "600000.gpio", 837, 0, "P8_37A", "", "", -1},
			{map[int]int{128: 111}, map[int]string{128: "P8.33B"}, "600000.gpio", 833, 0, "P8_33B", "", "", -1},
			{map[int]int{128: 115}, map[int]string{128: "P9.17B"}, "600000.gpio", 917, 0, "P9_17B", "", "", -1},
			{map[int]int{128: 116}, map[int]string{128: "P8.35B"}, "600000.gpio", 835, 0, "P8_35B", "", "", -1},
			{map[int]int{128: 118}, map[int]string{128: "P9.26A"}, "600000.gpio", 926, 0, "P9_26A", "", "", -1},
			{map[int]int{128: 119}, map[int]string{128: "P9.24A"}, "600000.gpio", 924, 0, "P9_24A", "", "", -1},
			{map[int]int{128: 120}, map[int]string{128: "P9.18"}, "600000.gpio", 918, 0, "P9_18B", "", "", -1},
			{map[int]int{128: 123}, map[int]string{128: "P9.42A"}, "600000.gpio", 942, 0, "P9_42A", "", "", -1},
			{map[int]int{128: 124}, map[int]string{128: "P9.27B"}, "600000.gpio", 927, 0, "P9_27B", "", "", -1},
			{map[int]int{128: 127}, map[int]string{128: "P9.25A"}, "600000.gpio", 925, 0, "P9_25A", "", "", -1},
			/// 601000.gpio dir 36 lines
			{map[int]int{36: 0}, map[int]string{36: "P9.41"}, "601000.gpio", 0, 0, "P9_41", "", "", -1},
			{map[int]int{36: 1}, map[int]string{36: "P9.19A"}, "601000.gpio", 0, 0, "P9_19A", "", "", -1},
			{map[int]int{36: 2}, map[int]string{36: "P9.20A"}, "601000.gpio", 0, 0, "P9_20A", "", "", -1},
			{map[int]int{36: 11}, map[int]string{36: "P9.28A"}, "601000.gpio", 0, 0, "P9_28A", "", "", -1},
			{map[int]int{36: 12}, map[int]string{36: "P9.31A"}, "601000.gpio", 0, 0, "P9_31A", "", "", -1},
			{map[int]int{36: 13}, map[int]string{36: "P9.30A"}, "601000.gpio", 0, 0, "P9_30A", "", "", -1},
			{map[int]int{36: 14}, map[int]string{36: "P9.29A"}, "601000.gpio", 0, 0, "P9_29A", "", "", -1},
		},
		Compats: []string{"bb,Ai64"},
	},
}

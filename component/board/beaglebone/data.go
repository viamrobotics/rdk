package bb

import "go.viam.com/rdk/component/board/commonsysfs"

const bbAi = "bb_Ai64"

var boardInfoMappings = map[string]commonsysfs.BoardInformation{
	bbAi: {
		PinDefinitions: []commonsysfs.PinDefinition{
			// {GPIOChipRelativeIDs: map[int]int{128: 1}, GPIONames: map[int]string{}, GPIOChipSysFSDir: "600000.gpio", PinNumberBoard: 0, PinNumberBCM: 0, PinNameCVM: "P9_11", PinNameTegraSOC: "", PWMChipSysFSDir: "", PWMID: -1 for no},
			// 600000.gpio dir 128 lines

			// TODO: Which pwm dirs are which:
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip0/device -> ../../../3000000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip10/device -> ../../../3050000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip2/device -> ../../../3010000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip4/device -> ../../../3020000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip6/device -> ../../../3030000.pwm
			// lrwxrwxrwx 1 root root 0 Jan  1  1970 /sys/class/pwm/pwmchip8/device -> ../../../3040000.pwm
			{map[int]int{128: 1}, map[int]string{}, "600000.gpio", 0, 0, "P9_11", "", "", -1},
			{map[int]int{128: 2}, map[int]string{}, "600000.gpio", 0, 0, "P9_13", "", "", -1},
			{map[int]int{128: 3}, map[int]string{}, "600000.gpio", 0, 0, "P8_17", "", "", -1},
			{map[int]int{128: 4}, map[int]string{}, "600000.gpio", 0, 0, "P8_18", "", "", -1},
			{map[int]int{128: 5}, map[int]string{}, "600000.gpio", 0, 0, "P8_22", "", "", -1},
			{map[int]int{128: 6}, map[int]string{}, "600000.gpio", 0, 0, "P8_24", "", "", -1},
			{map[int]int{128: 7}, map[int]string{}, "600000.gpio", 0, 0, "P8_34", "", "", -1},
			{map[int]int{128: 8}, map[int]string{}, "600000.gpio", 0, 0, "P8_36", "", "", -1},
			{map[int]int{128: 9}, map[int]string{}, "600000.gpio", 0, 0, "P8_38A", "", "", -1},
			{map[int]int{128: 10}, map[int]string{}, "600000.gpio", 0, 0, "P9_23", "", "", -1},
			{map[int]int{128: 11}, map[int]string{}, "600000.gpio", 0, 0, "P8_37B", "", "", -1},
			{map[int]int{128: 12}, map[int]string{}, "600000.gpio", 0, 0, "P9_26B", "", "", -1},
			{map[int]int{128: 13}, map[int]string{}, "600000.gpio", 0, 0, "P9_24B", "", "", -1},
			{map[int]int{128: 14}, map[int]string{}, "600000.gpio", 0, 0, "P8_08", "", "", 1}, // pwm?
			{map[int]int{128: 15}, map[int]string{}, "600000.gpio", 0, 0, "P8_07", "", "", 1}, // pwm?
			{map[int]int{128: 16}, map[int]string{}, "600000.gpio", 0, 0, "P8_10", "", "", 1}, // pwm?
			{map[int]int{128: 17}, map[int]string{}, "600000.gpio", 0, 0, "P8_09", "", "", 1}, // pwm?
			{map[int]int{128: 18}, map[int]string{}, "600000.gpio", 0, 0, "P9_42B", "", "", -1},
			{map[int]int{128: 20}, map[int]string{}, "600000.gpio", 0, 0, "P8_03", "", "", -1},
			{map[int]int{128: 24}, map[int]string{}, "600000.gpio", 0, 0, "P8_35A", "", "", -1},
			{map[int]int{128: 25}, map[int]string{}, "600000.gpio", 0, 0, "P8_33A", "", "", -1},
			{map[int]int{128: 26}, map[int]string{}, "600000.gpio", 0, 0, "P8_32A", "", "", -1},
			{map[int]int{128: 28}, map[int]string{}, "600000.gpio", 0, 0, "P9_17A", "", "", -1},
			{map[int]int{128: 30}, map[int]string{}, "600000.gpio", 0, 0, "P8_21", "", "", -1},
			{map[int]int{128: 31}, map[int]string{}, "600000.gpio", 0, 0, "P8_23", "", "", -1},
			{map[int]int{128: 32}, map[int]string{}, "600000.gpio", 0, 0, "P8_31A", "", "", -1},
			{map[int]int{128: 33}, map[int]string{}, "600000.gpio", 0, 0, "P8_05", "", "", -1},
			{map[int]int{128: 34}, map[int]string{}, "600000.gpio", 0, 0, "P8_06", "", "", -1},
			{map[int]int{128: 35}, map[int]string{}, "600000.gpio", 0, 0, "P8_25", "", "", -1},
			{map[int]int{128: 38}, map[int]string{}, "600000.gpio", 0, 0, "P9_22A (BOOTMODE1)", "", "", -1},
			{map[int]int{128: 39}, map[int]string{}, "600000.gpio", 0, 0, "P9_21A", "", "", -1},
			{map[int]int{128: 40}, map[int]string{}, "600000.gpio", 0, 0, "P9_18A", "", "", -1},
			{map[int]int{128: 43}, map[int]string{}, "600000.gpio", 0, 0, "P9_28B", "", "", -1},
			{map[int]int{128: 44}, map[int]string{}, "600000.gpio", 0, 0, "P9_30B", "", "", -1},
			{map[int]int{128: 45}, map[int]string{}, "600000.gpio", 0, 0, "P9_12", "", "", -1},
			{map[int]int{128: 46}, map[int]string{}, "600000.gpio", 0, 0, "P9_27A", "", "", -1},
			{map[int]int{128: 47}, map[int]string{}, "600000.gpio", 0, 0, "P9_15", "", "", -1},
			{map[int]int{128: 48}, map[int]string{}, "600000.gpio", 0, 0, "P8_04 (BOOTMODE2)", "", "", -1},
			{map[int]int{128: 50}, map[int]string{}, "600000.gpio", 0, 0, "P9_33B", "", "", -1},
			{map[int]int{128: 51}, map[int]string{}, "600000.gpio", 0, 0, "P8_26", "", "", -1},
			{map[int]int{128: 52}, map[int]string{}, "600000.gpio", 0, 0, "P9_31B", "", "", -1},
			{map[int]int{128: 53}, map[int]string{}, "600000.gpio", 0, 0, "P9_29B", "", "", -1},
			{map[int]int{128: 54}, map[int]string{}, "600000.gpio", 0, 0, "P9_39B", "", "", -1},
			{map[int]int{128: 55}, map[int]string{}, "600000.gpio", 0, 0, "P9_35B", "", "", -1},
			{map[int]int{128: 56}, map[int]string{}, "600000.gpio", 0, 0, "P9_36B", "", "", -1},
			{map[int]int{128: 57}, map[int]string{}, "600000.gpio", 0, 0, "P9_37B", "", "", -1},
			{map[int]int{128: 58}, map[int]string{}, "600000.gpio", 0, 0, "P9_38B", "", "", -1},
			{map[int]int{128: 59}, map[int]string{}, "600000.gpio", 0, 0, "P8_12", "", "", -1},
			{map[int]int{128: 60}, map[int]string{}, "600000.gpio", 0, 0, "P8_11 (BOOTMODE7)", "", "", -1},
			{map[int]int{128: 61}, map[int]string{}, "600000.gpio", 0, 0, "P8_15", "", "", -1},
			{map[int]int{128: 62}, map[int]string{}, "600000.gpio", 0, 0, "P8_16", "", "", -1},
			{map[int]int{128: 65}, map[int]string{}, "600000.gpio", 0, 0, "P8_43", "", "", -1},
			{map[int]int{128: 66}, map[int]string{}, "600000.gpio", 0, 0, "P8_44", "", "", -1},
			{map[int]int{128: 67}, map[int]string{}, "600000.gpio", 0, 0, "P8_41", "", "", -1},
			{map[int]int{128: 68}, map[int]string{}, "600000.gpio", 0, 0, "P8_42 (BOOTMODE6)", "", "", -1},
			{map[int]int{128: 69}, map[int]string{}, "600000.gpio", 0, 0, "P8_39", "", "", -1},
			{map[int]int{128: 70}, map[int]string{}, "600000.gpio", 0, 0, "P8_40", "", "", -1},
			{map[int]int{128: 71}, map[int]string{}, "600000.gpio", 0, 0, "P8_27", "", "", -1},
			{map[int]int{128: 72}, map[int]string{}, "600000.gpio", 0, 0, "P8_28", "", "", -1},
			{map[int]int{128: 73}, map[int]string{}, "600000.gpio", 0, 0, "P8_29", "", "", -1},
			{map[int]int{128: 74}, map[int]string{}, "600000.gpio", 0, 0, "P8_30", "", "", -1},
			{map[int]int{128: 75}, map[int]string{}, "600000.gpio", 0, 0, "P8_14", "", "", -1},
			{map[int]int{128: 76}, map[int]string{}, "600000.gpio", 0, 0, "P8_20", "", "", -1},
			{map[int]int{128: 77}, map[int]string{}, "600000.gpio", 0, 0, "P9_20B", "", "", -1},
			{map[int]int{128: 78}, map[int]string{}, "600000.gpio", 0, 0, "P9_19B", "", "", -1},
			{map[int]int{128: 79}, map[int]string{}, "600000.gpio", 0, 0, "P8_45", "", "", -1},
			{map[int]int{128: 80}, map[int]string{}, "600000.gpio", 0, 0, "P8_46 (BOOTMODE3)", "", "", -1},
			{map[int]int{128: 81}, map[int]string{}, "600000.gpio", 0, 0, "P9_40B", "", "", -1},
			{map[int]int{128: 88}, map[int]string{}, "600000.gpio", 0, 0, "P8_19", "", "", -1},
			{map[int]int{128: 89}, map[int]string{}, "600000.gpio", 0, 0, "P8_13", "", "", -1},
			{map[int]int{128: 90}, map[int]string{}, "600000.gpio", 0, 0, "P9_21B", "", "", -1},
			{map[int]int{128: 91}, map[int]string{}, "600000.gpio", 0, 0, "P9_22B", "", "", -1},
			{map[int]int{128: 93}, map[int]string{}, "600000.gpio", 0, 0, "P9_14", "", "", -1},
			{map[int]int{128: 94}, map[int]string{}, "600000.gpio", 0, 0, "P9_16", "", "", -1},
			{map[int]int{128: 104}, map[int]string{}, "600000.gpio", 0, 0, "P9_25B", "", "", -1},
			{map[int]int{128: 105}, map[int]string{}, "600000.gpio", 0, 0, "P8_38B", "", "", -1},
			{map[int]int{128: 106}, map[int]string{}, "600000.gpio", 0, 0, "P8_37A", "", "", -1},
			{map[int]int{128: 111}, map[int]string{}, "600000.gpio", 0, 0, "P8_33B", "", "", -1},
			{map[int]int{128: 115}, map[int]string{}, "600000.gpio", 0, 0, "P9_17B", "", "", -1},
			{map[int]int{128: 116}, map[int]string{}, "600000.gpio", 0, 0, "P8_35B", "", "", -1},
			{map[int]int{128: 118}, map[int]string{}, "600000.gpio", 0, 0, "P9_26A", "", "", -1},
			{map[int]int{128: 119}, map[int]string{}, "600000.gpio", 0, 0, "P9_24A", "", "", -1},
			{map[int]int{128: 120}, map[int]string{}, "600000.gpio", 0, 0, "P9_18B", "", "", -1},
			{map[int]int{128: 123}, map[int]string{}, "600000.gpio", 0, 0, "P9_42A", "", "", -1},
			{map[int]int{128: 124}, map[int]string{}, "600000.gpio", 0, 0, "P9_27B", "", "", -1},
			{map[int]int{128: 127}, map[int]string{}, "600000.gpio", 0, 0, "P9_25A", "", "", -1},
			/// 601000.gpio dir 36 lines
			{map[int]int{36: 0}, map[int]string{}, "601000.gpio", 0, 0, "P9_41", "", "", -1},
			{map[int]int{36: 1}, map[int]string{}, "601000.gpio", 0, 0, "P9_19A", "", "", -1},
			{map[int]int{36: 2}, map[int]string{}, "601000.gpio", 0, 0, "P9_20A", "", "", -1},
			{map[int]int{36: 11}, map[int]string{}, "601000.gpio", 0, 0, "P9_28A", "", "", -1},
			{map[int]int{36: 12}, map[int]string{}, "601000.gpio", 0, 0, "P9_31A", "", "", -1},
			{map[int]int{36: 13}, map[int]string{}, "601000.gpio", 0, 0, "P9_30A", "", "", -1},
			{map[int]int{36: 14}, map[int]string{}, "601000.gpio", 0, 0, "P9_29A", "", "", -1},
		},
		Compats: []string{"bb,Ai64"},
	},
}

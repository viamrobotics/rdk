package ti

import "go.viam.com/rdk/component/board/commonsysfs"

const tiTDA4VM = "ti_tda4vm"

var boardInfoMappings = map[string]commonsysfs.BoardInformation{
	tiTDA4VM: {
		[]commonsysfs.PinDefinition{
			{map[int]int{128: 84}, map[int]string{}, "600000.gpio", 3, 2, "GPIO0_84", "", "", -1},
			{map[int]int{128: 83}, map[int]string{}, "600000.gpio", 5, 3, "GPIO0_83", "", "", -1},
			{map[int]int{128: 7}, map[int]string{}, "600000.gpio", 7, 4, "GPIO0_7", "", "", -1},
			{map[int]int{128: 70}, map[int]string{}, "600000.gpio", 8, 14, "GPIO0_70", "", "", -1},
			{map[int]int{128: 81}, map[int]string{}, "600000.gpio", 10, 15, "GPIO0_81", "", "", -1},
			{map[int]int{128: 71}, map[int]string{}, "600000.gpio", 11, 17, "GPIO0_71", "", "", -1},
			{map[int]int{128: 1}, map[int]string{}, "600000.gpio", 12, 18, "GPIO0_1", "", "", -1},
			{map[int]int{128: 82}, map[int]string{}, "600000.gpio", 13, 27, "GPIO0_82", "", "", -1},
			{map[int]int{128: 11}, map[int]string{}, "600000.gpio", 15, 22, "GPIO0_11", "", "", -1},
			{map[int]int{128: 5}, map[int]string{}, "600000.gpio", 16, 23, "GPIO0_5", "", "", -1},
			{map[int]int{36: 12}, map[int]string{}, "601000.gpio", 18, 24, "GPIO0_12", "", "", -1},
			{map[int]int{128: 101}, map[int]string{}, "600000.gpio", 19, 10, "GPIO0_101", "", "", -1},
			{map[int]int{128: 107}, map[int]string{}, "600000.gpio", 21, 9, "GPIO0_107", "", "", -1},
			{map[int]int{128: 8}, map[int]string{}, "600000.gpio", 22, 25, "GPIO0_8", "", "", -1},
			{map[int]int{128: 103}, map[int]string{}, "600000.gpio", 23, 11, "GPIO0_103", "", "", -1},
			{map[int]int{128: 102}, map[int]string{}, "600000.gpio", 24, 8, "GPIO0_102", "", "", -1},
			{map[int]int{128: 108}, map[int]string{}, "600000.gpio", 26, 7, "GPIO0_108", "", "", -1},
			{map[int]int{128: 93}, map[int]string{}, "600000.gpio", 29, 5, "GPIO0_93", "", "3020000.pwm", 0},
			{map[int]int{128: 94}, map[int]string{}, "600000.gpio", 31, 6, "GPIO0_94", "", "3020000.pwm", 1},
			{map[int]int{128: 98}, map[int]string{}, "600000.gpio", 32, 12, "GPIO0_98", "", "3030000.pwm", 0},
			{map[int]int{128: 99}, map[int]string{}, "600000.gpio", 33, 13, "GPIO0_99", "", "3030000.pwm", 1},
			{map[int]int{128: 2}, map[int]string{}, "600000.gpio", 35, 19, "GPIO0_2", "", "", -1},
			{map[int]int{128: 97}, map[int]string{}, "600000.gpio", 36, 16, "GPIO0_97", "", "", -1},
			{map[int]int{128: 115}, map[int]string{}, "600000.gpio", 37, 26, "GPIO0_115", "", "", -1},
			{map[int]int{128: 3}, map[int]string{}, "600000.gpio", 38, 20, "GPIO0_3", "", "", -1},
			{map[int]int{128: 4}, map[int]string{}, "600000.gpio", 40, 21, "GPIO0_4", "", "", -1},
		},
		[]string{"ti,j721e-sk", "ti,j721e"},
	},
}

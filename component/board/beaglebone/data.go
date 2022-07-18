package bb

import "go.viam.com/rdk/component/board/commonsysfs"

const bbAi = "bb_Ai64"

var boardInfoMappings = map[string]commonsysfs.BoardInformation{
	bbAi: {
		[]commonsysfs.PinDefinition{
			{map[int]int{128: 84}, map[int]string{}, "600000.gpio", 3, 2, "GPIO0_84", "", "", -1},
		},
		[]string{"bb,Ai64"},
	},
}

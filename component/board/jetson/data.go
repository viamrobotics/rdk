package jetson

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// adapted from https://github.com/NVIDIA/jetson-gpio (MIT License)

const (
	claraAGXXavier = "clara_agx_xavier"
	jetsonNX       = "jetson_nx"
	jetsonXavier   = "jetson_xavier"
	jetsonTX2      = "jetson_tx2"
	jetsonTX1      = "jetson_tx1"
	jetsonNano     = "jetson_nano"
	jetsonTX2NX    = "jetson_tx2_NX"
	tiTDA4VM       = "ti_tda4vm"
)

type pinDefinition struct {
	GPIOChipRelativeIDs map[int]int    // ngpio -> relative id
	GPIONames           map[int]string // e.g. ngpio=169=PQ.06 for claraAGXXavier
	GPIOChipSysFSDir    string
	PinNumberBoard      int
	PinNumberBCM        int
	PinNameCVM          string
	PinNameTegraSOC     string
	PWMChipSysFSDir     string // empty for none
	PWMID               int    // -1 for none
}

var boardInfoMappings = map[string]struct {
	PinDefinitions []pinDefinition
	Compats        []string
}{
	claraAGXXavier: {
		[]pinDefinition{
			{map[int]int{224: 134, 169: 106}, map[int]string{169: "PQ.06"}, "2200000.gpio", 7, 4, "MCLK05", "SOC_GPIO42", "", -1},
			{map[int]int{224: 140, 169: 112}, map[int]string{169: "PR.04"}, "2200000.gpio", 11, 17, "UART1_RTS", "UART1_RTS", "", -1},
			{map[int]int{224: 63, 169: 51}, map[int]string{169: "PH.07"}, "2200000.gpio", 12, 18, "I2S2_CLK", "DAP2_SCLK", "", -1},
			{map[int]int{224: 124, 169: 96}, map[int]string{169: "PP.04"}, "2200000.gpio", 13, 27, "GPIO32", "SOC_GPIO04", "", -1},
			//  Older versions of L4T don"t enable this PWM controller in DT, so this PWM
			// channel may not be available.
			{
				map[int]int{224: 105, 169: 84},
				map[int]string{169: "PN.01"},
				"2200000.gpio",
				15,
				22,
				"GPIO27",
				"SOC_GPIO54",
				"3280000.pwm",
				0,
			},
			{map[int]int{40: 8, 30: 8}, map[int]string{30: "PBB.00"}, "c2f0000.gpio", 16, 23, "GPIO8", "CAN1_STB", "", -1},
			{
				map[int]int{224: 56, 169: 44},
				map[int]string{169: "PH.00"},
				"2200000.gpio",
				18,
				24,
				"GPIO35",
				"SOC_GPIO12",
				"32c0000.pwm",
				0,
			},
			{map[int]int{224: 205, 169: 162}, map[int]string{169: "PZ.05"}, "2200000.gpio", 19, 10, "SPI1_MOSI", "SPI1_MOSI", "", -1},
			{map[int]int{224: 204, 169: 161}, map[int]string{169: "PZ.04"}, "2200000.gpio", 21, 9, "SPI1_MISO", "SPI1_MISO", "", -1},
			{map[int]int{224: 129, 169: 101}, map[int]string{169: "PQ.01"}, "2200000.gpio", 22, 25, "GPIO17", "SOC_GPIO21", "", -1},
			{map[int]int{224: 203, 169: 160}, map[int]string{169: "PZ.03"}, "2200000.gpio", 23, 11, "SPI1_CLK", "SPI1_SCK", "", -1},
			{map[int]int{224: 206, 169: 163}, map[int]string{169: "PZ.06"}, "2200000.gpio", 24, 8, "SPI1_CS0_N", "SPI1_CS0_N", "", -1},
			{map[int]int{224: 207, 169: 164}, map[int]string{169: "PZ.07"}, "2200000.gpio", 26, 7, "SPI1_CS1_N", "SPI1_CS1_N", "", -1},
			{map[int]int{40: 3, 30: 3}, map[int]string{30: "PAA.03"}, "c2f0000.gpio", 29, 5, "CAN0_DIN", "CAN0_DIN", "", -1},
			{map[int]int{40: 2, 30: 2}, map[int]string{30: "PAA.02"}, "c2f0000.gpio", 31, 6, "CAN0_DOUT", "CAN0_DOUT", "", -1},
			{map[int]int{40: 9, 30: 9}, map[int]string{30: "PBB.01"}, "c2f0000.gpio", 32, 12, "GPIO9", "CAN1_EN", "", -1},
			{map[int]int{40: 0, 30: 0}, map[int]string{30: "PAA.00"}, "c2f0000.gpio", 33, 13, "CAN1_DOUT", "CAN1_DOUT", "", -1},
			{map[int]int{224: 66, 169: 54}, map[int]string{169: "PI.02"}, "2200000.gpio", 35, 19, "I2S2_FS", "DAP2_FS", "", -1},
			// Input-only (due to base board)
			{map[int]int{224: 141, 169: 113}, map[int]string{169: "PR.05"}, "2200000.gpio", 36, 16, "UART1_CTS", "UART1_CTS", "", -1},
			{map[int]int{40: 1, 30: 1}, map[int]string{30: "PAA.01"}, "c2f0000.gpio", 37, 26, "CAN1_DIN", "CAN1_DIN", "", -1},
			{map[int]int{224: 65, 169: 53}, map[int]string{169: "PI.01"}, "2200000.gpio", 38, 20, "I2S2_DIN", "DAP2_DIN", "", -1},
			{map[int]int{224: 64, 169: 52}, map[int]string{169: "PI.00"}, "2200000.gpio", 40, 21, "I2S2_DOUT", "DAP2_DOUT", "", -1},
		},
		[]string{"nvidia,e3900-0000+p2888-0004"},
	},
	tiTDA4VM: {
		[]pinDefinition{
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
	//nolint:dupl
	jetsonNX: {
		[]pinDefinition{
			{map[int]int{224: 148, 169: 118}, map[int]string{169: "PS.04"}, "2200000.gpio", 7, 4, "GPIO09", "AUD_MCLK", "", -1},
			{map[int]int{224: 140, 169: 112}, map[int]string{169: "PR.04"}, "2200000.gpio", 11, 17, "UART1_RTS", "UART1_RTS", "", -1},
			{map[int]int{224: 157, 169: 127}, map[int]string{169: "PT.05"}, "2200000.gpio", 12, 18, "I2S0_SCLK", "DAP5_SCLK", "", -1},
			{map[int]int{224: 192, 169: 149}, map[int]string{169: "PY.00"}, "2200000.gpio", 13, 27, "SPI1_SCK", "SPI3_SCK", "", -1},
			{map[int]int{40: 20, 30: 16}, map[int]string{30: "PCC.04"}, "c2f0000.gpio", 15, 22, "GPIO12", "TOUCH_CLK", "", -1},
			{map[int]int{224: 196, 169: 153}, map[int]string{169: "PY.04"}, "2200000.gpio", 16, 23, "SPI1_CS1", "SPI3_CS1_N", "", -1},
			{map[int]int{224: 195, 169: 152}, map[int]string{169: "PY.03"}, "2200000.gpio", 18, 24, "SPI1_CS0", "SPI3_CS0_N", "", -1},
			{map[int]int{224: 205, 169: 162}, map[int]string{169: "PZ.05"}, "2200000.gpio", 19, 10, "SPI0_MOSI", "SPI1_MOSI", "", -1},
			{map[int]int{224: 204, 169: 161}, map[int]string{169: "PZ.04"}, "2200000.gpio", 21, 9, "SPI0_MISO", "SPI1_MISO", "", -1},
			{map[int]int{224: 193, 169: 150}, map[int]string{169: "PY.01"}, "2200000.gpio", 22, 25, "SPI1_MISO", "SPI3_MISO", "", -1},
			{map[int]int{224: 203, 169: 160}, map[int]string{169: "PZ.03"}, "2200000.gpio", 23, 11, "SPI0_SCK", "SPI1_SCK", "", -1},
			{map[int]int{224: 206, 169: 163}, map[int]string{169: "PZ.06"}, "2200000.gpio", 24, 8, "SPI0_CS0", "SPI1_CS0_N", "", -1},
			{map[int]int{224: 207, 169: 164}, map[int]string{169: "PZ.07"}, "2200000.gpio", 26, 7, "SPI0_CS1", "SPI1_CS1_N", "", -1},
			{map[int]int{224: 133, 169: 105}, map[int]string{169: "PQ.05"}, "2200000.gpio", 29, 5, "GPIO01", "SOC_GPIO41", "", -1},
			{map[int]int{224: 134, 169: 106}, map[int]string{169: "PQ.06"}, "2200000.gpio", 31, 6, "GPIO11", "SOC_GPIO42", "", -1},
			{
				map[int]int{224: 136, 169: 108},
				map[int]string{169: "PR.00"},
				"2200000.gpio",
				32,
				12,
				"GPIO07",
				"SOC_GPIO44",
				"32f0000.pwm",
				0,
			},
			{
				map[int]int{224: 105, 169: 84},
				map[int]string{169: "PN.01"},
				"2200000.gpio",
				33,
				13,
				"GPIO13",
				"SOC_GPIO54",
				"3280000.pwm",
				0,
			},
			{map[int]int{224: 160, 169: 130}, map[int]string{169: "PU.00"}, "2200000.gpio", 35, 19, "I2S0_FS", "DAP5_FS", "", -1},
			{map[int]int{224: 141, 169: 113}, map[int]string{169: "PR.05"}, "2200000.gpio", 36, 16, "UART1_CTS", "UART1_CTS", "", -1},
			{map[int]int{224: 194, 169: 151}, map[int]string{169: "PY.02"}, "2200000.gpio", 37, 26, "SPI1_MOSI", "SPI3_MOSI", "", -1},
			{map[int]int{224: 159, 169: 129}, map[int]string{169: "PT.07"}, "2200000.gpio", 38, 20, "I2S0_DIN", "DAP5_DIN", "", -1},
			{map[int]int{224: 158, 169: 128}, map[int]string{169: "PT.06"}, "2200000.gpio", 40, 21, "I2S0_DOUT", "DAP5_DOUT", "", -1},
		},
		[]string{
			"nvidia,p3509-0000+p3668-0000",
			"nvidia,p3509-0000+p3668-0001",
			"nvidia,p3449-0000+p3668-0000",
			"nvidia,p3449-0000+p3668-0001",
		},
	},
	jetsonXavier: {
		[]pinDefinition{
			{map[int]int{224: 134, 169: 106}, map[int]string{169: "PQ.06"}, "2200000.gpio", 7, 4, "MCLK05", "SOC_GPIO42", "", -1},
			{map[int]int{224: 140, 169: 112}, map[int]string{169: "PR.04"}, "2200000.gpio", 11, 17, "UART1_RTS", "UART1_RTS", "", -1},
			{map[int]int{224: 63, 169: 51}, map[int]string{169: "PH.07"}, "2200000.gpio", 12, 18, "I2S2_CLK", "DAP2_SCLK", "", -1},
			{
				map[int]int{224: 136, 169: 108},
				map[int]string{169: "PR.00"},
				"2200000.gpio",
				13,
				27,
				"PWM01",
				"SOC_GPIO44",
				"32f0000.pwm",
				0,
			},
			// Older versions of L4T don't enable this PWM controller in DT, so this PWM
			// channel may not be available.
			{
				map[int]int{224: 105, 169: 84},
				map[int]string{169: "PN.01"},
				"2200000.gpio",
				15,
				22,
				"GPIO27",
				"SOC_GPIO54",
				"3280000.pwm",
				0,
			},
			{map[int]int{40: 8, 30: 8}, map[int]string{30: "PBB.00"}, "c2f0000.gpio", 16, 23, "GPIO8", "CAN1_STB", "", -1},
			{
				map[int]int{224: 56, 169: 44},
				map[int]string{169: "PH.00"},
				"2200000.gpio",
				18,
				24,
				"GPIO35",
				"SOC_GPIO12",
				"32c0000.pwm",
				0,
			},
			{map[int]int{224: 205, 169: 162}, map[int]string{169: "PZ.05"}, "2200000.gpio", 19, 10, "SPI1_MOSI", "SPI1_MOSI", "", -1},
			{map[int]int{224: 204, 169: 161}, map[int]string{169: "PZ.04"}, "2200000.gpio", 21, 9, "SPI1_MISO", "SPI1_MISO", "", -1},
			{map[int]int{224: 129, 169: 101}, map[int]string{169: "PQ.01"}, "2200000.gpio", 22, 25, "GPIO17", "SOC_GPIO21", "", -1},
			{map[int]int{224: 203, 169: 160}, map[int]string{169: "PZ.03"}, "2200000.gpio", 23, 11, "SPI1_CLK", "SPI1_SCK", "", -1},
			{map[int]int{224: 206, 169: 163}, map[int]string{169: "PZ.06"}, "2200000.gpio", 24, 8, "SPI1_CS0_N", "SPI1_CS0_N", "", -1},
			{map[int]int{224: 207, 169: 164}, map[int]string{169: "PZ.07"}, "2200000.gpio", 26, 7, "SPI1_CS1_N", "SPI1_CS1_N", "", -1},
			{map[int]int{40: 3, 30: 3}, map[int]string{30: "PAA.03"}, "c2f0000.gpio", 29, 5, "CAN0_DIN", "CAN0_DIN", "", -1},
			{map[int]int{40: 2, 30: 2}, map[int]string{30: "PAA.02"}, "c2f0000.gpio", 31, 6, "CAN0_DOUT", "CAN0_DOUT", "", -1},
			{map[int]int{40: 9, 30: 9}, map[int]string{30: "PBB.01"}, "c2f0000.gpio", 32, 12, "GPIO9", "CAN1_EN", "", -1},
			{map[int]int{40: 0, 30: 0}, map[int]string{30: "PAA.00"}, "c2f0000.gpio", 33, 13, "CAN1_DOUT", "CAN1_DOUT", "", -1},
			{map[int]int{224: 66, 169: 54}, map[int]string{169: "PI.02"}, "2200000.gpio", 35, 19, "I2S2_FS", "DAP2_FS", "", -1},
			// Input-only (due to base board)
			{map[int]int{224: 141, 169: 113}, map[int]string{169: "PR.05"}, "2200000.gpio", 36, 16, "UART1_CTS", "UART1_CTS", "", -1},
			{map[int]int{40: 1, 30: 1}, map[int]string{30: "PAA.01"}, "c2f0000.gpio", 37, 26, "CAN1_DIN", "CAN1_DIN", "", -1},
			{map[int]int{224: 65, 169: 53}, map[int]string{169: "PI.01"}, "2200000.gpio", 38, 20, "I2S2_DIN", "DAP2_DIN", "", -1},
			{map[int]int{224: 64, 169: 52}, map[int]string{169: "PI.00"}, "2200000.gpio", 40, 21, "I2S2_DOUT", "DAP2_DOUT", "", -1},
		},
		[]string{
			"nvidia,p2972-0000",
			"nvidia,p2972-0006",
			"nvidia,jetson-xavier",
			"nvidia,galen-industrial",
			"nvidia,jetson-xavier-industrial",
		},
	},
	//nolint:dupl
	jetsonTX2NX: {
		[]pinDefinition{
			{map[int]int{192: 76, 140: 66}, map[int]string{140: "PJ.04"}, "2200000.gpio", 7, 4, "GPIO09", "AUD_MCLK", "", -1},
			{map[int]int{64: 28, 47: 23}, map[int]string{47: "PW.04"}, "c2f0000.gpio", 11, 17, "UART1_RTS", "UART3_RTS", "", -1},
			{map[int]int{192: 72, 140: 62}, map[int]string{140: "PJ.00"}, "2200000.gpio", 12, 18, "I2S0_SCLK", "DAP1_SCLK", "", -1},
			{map[int]int{64: 17, 47: 12}, map[int]string{47: "PV.01"}, "c2f0000.gpio", 13, 27, "SPI1_SCK", "GPIO_SEN1", "", -1},
			{map[int]int{192: 18, 140: 16}, map[int]string{140: "PC.02"}, "2200000.gpio", 15, 22, "GPIO12", "DAP2_DOUT", "", -1},
			{map[int]int{192: 19, 140: 17}, map[int]string{140: "PC.03"}, "2200000.gpio", 16, 23, "SPI1_CS1", "DAP2_DIN", "", -1},
			{map[int]int{64: 20, 47: 15}, map[int]string{47: "PV.04"}, "c2f0000.gpio", 18, 24, "SPI1_CS0", "GPIO_SEN4", "", -1},
			{map[int]int{192: 58, 140: 49}, map[int]string{140: "PH.02"}, "2200000.gpio", 19, 10, "SPI0_MOSI", "GPIO_WAN7", "", -1},
			{map[int]int{192: 57, 140: 48}, map[int]string{140: "PH.01"}, "2200000.gpio", 21, 9, "SPI0_MISO", "GPIO_WAN6", "", -1},
			{map[int]int{64: 18, 47: 13}, map[int]string{47: "PV.02"}, "c2f0000.gpio", 22, 25, "SPI1_MISO", "GPIO_SEN2", "", -1},
			{map[int]int{192: 56, 140: 47}, map[int]string{140: "PH.00"}, "2200000.gpio", 23, 11, "SPI1_CLK", "GPIO_WAN5", "", -1},
			{map[int]int{192: 59, 140: 50}, map[int]string{140: "PH.03"}, "2200000.gpio", 24, 8, "SPI0_CS0", "GPIO_WAN8", "", -1},
			{map[int]int{192: 163, 140: 130}, map[int]string{140: "PY.03"}, "2200000.gpio", 26, 7, "SPI0_CS1", "GPIO_MDM4", "", -1},
			{map[int]int{192: 105, 140: 86}, map[int]string{140: "PN.01"}, "2200000.gpio", 29, 5, "GPIO01", "GPIO_CAM2", "", -1},
			{map[int]int{64: 50, 47: 41}, map[int]string{47: "PEE.02"}, "c2f0000.gpio", 31, 6, "GPIO11", "TOUCH_CLK", "", -1},
			{map[int]int{64: 8, 47: 5}, map[int]string{47: "PU.00"}, "c2f0000.gpio", 32, 12, "GPIO07", "GPIO_DIS0", "3280000.pwm", 0},
			{map[int]int{64: 13, 47: 10}, map[int]string{47: "PU.05"}, "c2f0000.gpio", 33, 13, "GPIO13", "GPIO_DIS5", "32a0000.pwm", 0},
			{map[int]int{192: 75, 140: 65}, map[int]string{140: "PJ.03"}, "2200000.gpio", 35, 19, "I2S0_FS", "DAP1_FS", "", -1},
			{map[int]int{64: 29, 47: 24}, map[int]string{47: "PW.05"}, "c2f0000.gpio", 36, 16, "UART1_CTS", "UART3_CTS", "", -1},
			{map[int]int{64: 19, 47: 14}, map[int]string{47: "PV.03"}, "c2f0000.gpio", 37, 26, "SPI1_MOSI", "GPIO_SEN3", "", -1},
			{map[int]int{192: 74, 140: 64}, map[int]string{140: "PJ.02"}, "2200000.gpio", 38, 20, "I2S0_DIN", "DAP1_DIN", "", -1},
			{map[int]int{192: 73, 140: 63}, map[int]string{140: "PJ.01"}, "2200000.gpio", 40, 21, "I2S0_DOUT", "DAP1_DOUT", "", -1},
		},
		[]string{
			"nvidia,p3509-0000+p3636-0001",
		},
	},
	jetsonTX2: {
		[]pinDefinition{
			{map[int]int{192: 76, 140: 66}, map[int]string{140: "PJ.04"}, "2200000.gpio", 7, 4, "PAUDIO_MCLK", "AUD_MCLK", "", -1},
			// Output-only (due to base board)
			{map[int]int{192: 146, 140: 117}, map[int]string{140: "PT.02"}, "2200000.gpio", 11, 17, "PUART0_RTS", "UART1_RTS", "", -1},
			{map[int]int{192: 72, 140: 62}, map[int]string{140: "PJ.00"}, "2200000.gpio", 12, 18, "PI2S0_CLK", "DAP1_SCLK", "", -1},
			{
				map[int]int{192: 77, 140: 67},
				map[int]string{140: "PJ.05"},
				"2200000.gpio",
				13,
				27,
				"PGPIO20_AUD_INT",
				"GPIO_AUD0",
				"",
				-1,
			},
			{map[int]int{-1: 15}, nil, "3160000.i2c/i2c-0/0-0074", 15, 22, "GPIO_EXP_P17", "GPIO_EXP_P17", "", -1},
			// Input-only (due to module):
			{map[int]int{64: 40, 47: 31}, map[int]string{47: "PAA.00"}, "c2f0000.gpio", 16, 23, "AO_DMIC_IN_DAT", "CAN_GPIO0", "", -1},
			{
				map[int]int{192: 161, 140: 128},
				map[int]string{140: "PY.01"},
				"2200000.gpio",
				18,
				24,
				"GPIO16_MDM_WAKE_AP",
				"GPIO_MDM2",
				"",
				-1,
			},
			{map[int]int{192: 109, 140: 90}, map[int]string{140: "PN.05"}, "2200000.gpio", 19, 10, "SPI1_MOSI", "GPIO_CAM6", "", -1},
			{map[int]int{192: 108, 140: 89}, map[int]string{140: "PN.04"}, "2200000.gpio", 21, 9, "SPI1_MISO", "GPIO_CAM5", "", -1},
			{map[int]int{-1: 14}, nil, "3160000.i2c/i2c-0/0-0074", 22, 25, "GPIO_EXP_P16", "GPIO_EXP_P16", "", -1},
			{map[int]int{192: 107, 140: 88}, map[int]string{140: "PN.03"}, "2200000.gpio", 23, 11, "SPI1_CLK", "GPIO_CAM4", "", -1},
			{map[int]int{192: 110, 140: 91}, map[int]string{140: "PN.06"}, "2200000.gpio", 24, 8, "SPI1_CS0", "GPIO_CAM7", "", -1},
			// Board pin 26 is not available on this board
			{map[int]int{192: 78, 140: 68}, map[int]string{140: "PJ.06"}, "2200000.gpio", 29, 5, "GPIO19_AUD_RST", "GPIO_AUD1", "", -1},
			{map[int]int{64: 42, 47: 33}, map[int]string{47: "PAA.02"}, "c2f0000.gpio", 31, 6, "GPIO9_MOTION_INT", "CAN_GPIO2", "", -1},
			// Output-only (due to module):
			{map[int]int{64: 41, 47: 32}, map[int]string{47: "PAA.01"}, "c2f0000.gpio", 32, 12, "AO_DMIC_IN_CLK", "CAN_GPIO1", "", -1},
			{
				map[int]int{192: 69, 140: 59},
				map[int]string{140: "PI.05"},
				"2200000.gpio",
				33,
				13,
				"GPIO11_AP_WAKE_BT",
				"GPIO_PQ5",
				"",
				-1,
			},
			{map[int]int{192: 75, 140: 65}, map[int]string{140: "PJ.03"}, "2200000.gpio", 35, 19, "I2S0_LRCLK", "DAP1_FS", "", -1},
			// Input-only (due to base board) IF NVIDIA debug card NOT plugged in
			// Output-only (due to base board) IF NVIDIA debug card plugged in
			{map[int]int{192: 147, 140: 118}, map[int]string{140: "PT.03"}, "2200000.gpio", 36, 16, "UART0_CTS", "UART1_CTS", "", -1},
			{
				map[int]int{192: 68, 140: 58},
				map[int]string{140: "PI.04"},
				"2200000.gpio",
				37,
				26,
				"GPIO8_ALS_PROX_INT",
				"GPIO_PQ4",
				"",
				-1,
			},
			{map[int]int{192: 74, 140: 64}, map[int]string{140: "PJ.02"}, "2200000.gpio", 38, 20, "I2S0_SDIN", "DAP1_DIN", "", -1},
			{map[int]int{192: 73, 140: 63}, map[int]string{140: "PJ.01"}, "2200000.gpio", 40, 21, "I2S0_SDOUT", "DAP1_DOUT", "", -1},
		},
		[]string{
			"nvidia,p2771-0000",
			"nvidia,p2771-0888",
			"nvidia,p3489-0000",
			"nvidia,lightning",
			"nvidia,quill",
			"nvidia,storm",
		},
	},
	jetsonTX1: {
		[]pinDefinition{
			{map[int]int{-1: 216}, nil, "6000d000.gpio", 7, 4, "AUDIO_MCLK", "AUD_MCLK", "", -1},
			// Output-only (due to base board)
			{map[int]int{-1: 162}, nil, "6000d000.gpio", 11, 17, "UART0_RTS", "UART1_RTS", "", -1},
			{map[int]int{-1: 11}, nil, "6000d000.gpio", 12, 18, "I2S0_CLK", "DAP1_SCLK", "", -1},
			{map[int]int{-1: 38}, nil, "6000d000.gpio", 13, 27, "GPIO20_AUD_INT", "GPIO_PE6", "", -1},
			{map[int]int{-1: 15}, nil, "7000c400.i2c/i2c-1/1-0074", 15, 22, "GPIO_EXP_P17", "GPIO_EXP_P17", "", -1},
			{map[int]int{-1: 37}, nil, "6000d000.gpio", 16, 23, "AO_DMIC_IN_DAT", "DMIC3_DAT", "", -1},
			{map[int]int{-1: 184}, nil, "6000d000.gpio", 18, 24, "GPIO16_MDM_WAKE_AP", "MODEM_WAKE_AP", "", -1},
			{map[int]int{-1: 16}, nil, "6000d000.gpio", 19, 10, "SPI1_MOSI", "SPI1_MOSI", "", -1},
			{map[int]int{-1: 17}, nil, "6000d000.gpio", 21, 9, "SPI1_MISO", "SPI1_MISO", "", -1},
			{map[int]int{-1: 14}, nil, "7000c400.i2c/i2c-1/1-0074", 22, 25, "GPIO_EXP_P16", "GPIO_EXP_P16", "", -1},
			{map[int]int{-1: 18}, nil, "6000d000.gpio", 23, 11, "SPI1_CLK", "SPI1_SCK", "", -1},
			{map[int]int{-1: 19}, nil, "6000d000.gpio", 24, 8, "SPI1_CS0", "SPI1_CS0", "", -1},
			{map[int]int{-1: 20}, nil, "6000d000.gpio", 26, 7, "SPI1_CS1", "SPI1_CS1", "", -1},
			{map[int]int{-1: 219}, nil, "6000d000.gpio", 29, 5, "GPIO19_AUD_RST", "GPIO_X1_AUD", "", -1},
			{map[int]int{-1: 186}, nil, "6000d000.gpio", 31, 6, "GPIO9_MOTION_INT", "MOTION_INT", "", -1},
			{map[int]int{-1: 36}, nil, "6000d000.gpio", 32, 12, "AO_DMIC_IN_CLK", "DMIC3_CLK", "", -1},
			{map[int]int{-1: 63}, nil, "6000d000.gpio", 33, 13, "GPIO11_AP_WAKE_BT", "AP_WAKE_NFC", "", -1},
			{map[int]int{-1: 8}, nil, "6000d000.gpio", 35, 19, "I2S0_LRCLK", "DAP1_FS", "", -1},
			// Input-only (due to base board) IF NVIDIA debug card NOT plugged in
			// Input-only (due to base board) (always reads fixed value) IF NVIDIA debug card plugged in
			{map[int]int{-1: 163}, nil, "6000d000.gpio", 36, 16, "UART0_CTS", "UART1_CTS", "", -1},
			{map[int]int{-1: 187}, nil, "6000d000.gpio", 37, 26, "GPIO8_ALS_PROX_INT", "ALS_PROX_INT", "", -1},
			{map[int]int{-1: 9}, nil, "6000d000.gpio", 38, 20, "I2S0_SDIN", "DAP1_DIN", "", -1},
			{map[int]int{-1: 10}, nil, "6000d000.gpio", 40, 21, "I2S0_SDOUT", "DAP1_DOUT", "", -1},
		},
		[]string{
			"nvidia,p2371-2180",
			"nvidia,jetson-cv",
		},
	},
	jetsonNano: {
		[]pinDefinition{
			{map[int]int{-1: 216}, nil, "6000d000.gpio", 7, 4, "GPIO9", "AUD_MCLK", "", -1},
			{map[int]int{-1: 50}, nil, "6000d000.gpio", 11, 17, "UART1_RTS", "UART2_RTS", "", -1},
			{map[int]int{-1: 79}, nil, "6000d000.gpio", 12, 18, "I2S0_SCLK", "DAP4_SCLK", "", -1},
			{map[int]int{-1: 14}, nil, "6000d000.gpio", 13, 27, "SPI1_SCK", "SPI2_SCK", "", -1},
			{map[int]int{-1: 194}, nil, "6000d000.gpio", 15, 22, "GPIO12", "LCD_TE", "", -1},
			{map[int]int{-1: 232}, nil, "6000d000.gpio", 16, 23, "SPI1_CS1", "SPI2_CS1", "", -1},
			{map[int]int{-1: 15}, nil, "6000d000.gpio", 18, 24, "SPI1_CS0", "SPI2_CS0", "", -1},
			{map[int]int{-1: 16}, nil, "6000d000.gpio", 19, 10, "SPI0_MOSI", "SPI1_MOSI", "", -1},
			{map[int]int{-1: 17}, nil, "6000d000.gpio", 21, 9, "SPI0_MISO", "SPI1_MISO", "", -1},
			{map[int]int{-1: 13}, nil, "6000d000.gpio", 22, 25, "SPI1_MISO", "SPI2_MISO", "", -1},
			{map[int]int{-1: 18}, nil, "6000d000.gpio", 23, 11, "SPI0_SCK", "SPI1_SCK", "", -1},
			{map[int]int{-1: 19}, nil, "6000d000.gpio", 24, 8, "SPI0_CS0", "SPI1_CS0", "", -1},
			{map[int]int{-1: 20}, nil, "6000d000.gpio", 26, 7, "SPI0_CS1", "SPI1_CS1", "", -1},
			{map[int]int{-1: 149}, nil, "6000d000.gpio", 29, 5, "GPIO01", "CAM_AF_EN", "", -1},
			{map[int]int{-1: 200}, nil, "6000d000.gpio", 31, 6, "GPIO11", "GPIO_PZ0", "", -1},
			// Older versions of L4T have a DT bug which instantiates a bogus device
			// which prevents this library from using this PWM channel.
			{map[int]int{-1: 168}, nil, "6000d000.gpio", 32, 12, "GPIO07", "LCD_BL_PW", "7000a000.pwm", 0},
			{map[int]int{-1: 38}, nil, "6000d000.gpio", 33, 13, "GPIO13", "GPIO_PE6", "7000a000.pwm", 2},
			{map[int]int{-1: 76}, nil, "6000d000.gpio", 35, 19, "I2S0_FS", "DAP4_FS", "", -1},
			{map[int]int{-1: 51}, nil, "6000d000.gpio", 36, 16, "UART1_CTS", "UART2_CTS", "", -1},
			{map[int]int{-1: 12}, nil, "6000d000.gpio", 37, 26, "SPI1_MOSI", "SPI2_MOSI", "", -1},
			{map[int]int{-1: 77}, nil, "6000d000.gpio", 38, 20, "I2S0_DIN", "DAP4_DIN", "", -1},
			{map[int]int{-1: 78}, nil, "6000d000.gpio", 40, 21, "I2S0_DOUT", "DAP4_DOUT", "", -1},
		},
		[]string{
			"nvidia,p3450-0000",
			"nvidia,p3450-0002",
			"nvidia,jetson-nano",
		},
	},
}

type gpioBoardMapping struct {
	gpioChipDev    string
	gpio           int
	gpioGlobal     int
	hwPWMSupported bool
}

var errNoJetson = errors.New("could not determine Jetson model")

func getGPIOBoardMappings() (map[int]gpioBoardMapping, error) {
	const (
		compatiblePath = "/proc/device-tree/compatible"
		idsPath        = "/proc/device-tree/chosen/plugin-manager/ids"
	)

	compatiblesRd, err := ioutil.ReadFile(compatiblePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errNoJetson
		}
		return nil, err
	}
	compatibles := utils.NewStringSet(strings.Split(string(compatiblesRd), "\x00")...)

	var pinDefs []pinDefinition
	for _, info := range boardInfoMappings {
		for _, v := range info.Compats {
			if _, ok := compatibles[v]; ok {
				pinDefs = info.PinDefinitions
				break
			}
		}
	}

	if pinDefs == nil {
		return nil, errNoJetson
	}

	gpioChipDirs := map[string]string{}
	gpioChipBase := map[string]int{}
	gpioChipNgpio := map[string]int{}

	sysfsPrefixes := []string{"/sys/devices/", "/sys/devices/platform/", "/sys/devices/platform/bus@100000/"}

	// Get the GPIO chip offsets
	gpioChipNames := make(map[string]struct{}, len(pinDefs))
	for _, pinDef := range pinDefs {
		if pinDef.GPIOChipSysFSDir == "" {
			continue
		}
		gpioChipNames[pinDef.GPIOChipSysFSDir] = struct{}{}
	}
	for gpioChipName := range gpioChipNames {
		var gpioChipDir string
		for _, prefix := range sysfsPrefixes {
			d := prefix + gpioChipName
			fileInfo, err := os.Stat(d)
			if err != nil {
				continue
			}
			if fileInfo.IsDir() {
				gpioChipDir = d
				break
			}
		}
		if gpioChipDir == "" {
			return nil, errors.Errorf("cannot find GPIO chip %q", gpioChipName)
		}
		files, err := os.ReadDir(gpioChipDir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "gpiochip") {
				continue
			}
			gpioChipDirs[gpioChipName] = file.Name()
			break
		}

		gpioChipGPIODir := gpioChipDir + "/gpio"
		files, err = os.ReadDir(gpioChipGPIODir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "gpiochip") {
				continue
			}

			baseFn := filepath.Join(gpioChipGPIODir, file.Name(), "base")
			//nolint:gosec
			baseRd, err := ioutil.ReadFile(baseFn)
			if err != nil {
				return nil, err
			}
			baseParsed, err := strconv.ParseInt(strings.TrimSpace(string(baseRd)), 10, 64)
			if err != nil {
				return nil, err
			}
			gpioChipBase[gpioChipName] = int(baseParsed)

			ngpioFn := filepath.Join(gpioChipGPIODir, file.Name(), "ngpio")
			//nolint:gosec
			ngpioRd, err := ioutil.ReadFile(ngpioFn)
			if err != nil {
				return nil, err
			}
			ngpioParsed, err := strconv.ParseInt(strings.TrimSpace(string(ngpioRd)), 10, 64)
			if err != nil {
				return nil, err
			}
			gpioChipNgpio[gpioChipName] = int(ngpioParsed)
			break
		}
	}

	data := make(map[int]gpioBoardMapping, len(pinDefs))
	for _, pinDef := range pinDefs {
		key := pinDef.PinNumberBoard

		chipGPIONgpio := gpioChipNgpio[pinDef.GPIOChipSysFSDir]
		chipGPIOBase := gpioChipBase[pinDef.GPIOChipSysFSDir]
		chipRelativeID, ok := pinDef.GPIOChipRelativeIDs[chipGPIONgpio]
		if !ok {
			chipRelativeID = pinDef.GPIOChipRelativeIDs[-1]
		}

		data[key] = gpioBoardMapping{
			gpioChipDev:    gpioChipDirs[pinDef.GPIOChipSysFSDir],
			gpio:           chipRelativeID,
			gpioGlobal:     chipGPIOBase + chipRelativeID,
			hwPWMSupported: pinDef.PWMID != -1,
		}
	}

	return data, nil
}

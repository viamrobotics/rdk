package jetson

import (
	"go.viam.com/rdk/components/board/genericlinux"
)

const (
	claraAGXXavier = "clara_agx_xavier"
	jetsonNX       = "jetson_nx"
	jetsonXavier   = "jetson_xavier"
	jetsonTX2      = "jetson_tx2"
	jetsonTX1      = "jetson_tx1"
	jetsonNano     = "jetson_nano"
	jetsonTX2NX    = "jetson_tx2_NX"
	jetsonOrinAGX  = "jetson_orin_agx"
	jetsonOrinNX   = "jetson_orin_nx"
	jetsonOrinNano = "jetson_orin_nano"
)

var claraAGXXavierPins = []genericlinux.PinDefinition{
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
}

//nolint:dupl // This is not actually a duplicate of jetsonTX2NXPins despite what the linter thinks
var jetsonNXPins = []genericlinux.PinDefinition{
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
}

var jetsonXavierPins = []genericlinux.PinDefinition{
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
}

//nolint:dupl // This is not actually a duplicate of jetsonNXPins despite wht the linter thinks
var jetsonTX2NXPins = []genericlinux.PinDefinition{
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
}

var jetsonTX2Pins = []genericlinux.PinDefinition{
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
}

var jetsonTX1Pins = []genericlinux.PinDefinition{
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
}

var jetsonNanoPins = []genericlinux.PinDefinition{
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
}

// There are 6 pins whose Broadcom SOC channel is -1 (pins 3, 5, 8, 10, 27, and 28). We
// added these pin definitions ourselves because they're not present in
// https://github.com/NVIDIA/jetson-gpio/blob/master/lib/python/Jetson/GPIO/gpio_pin_data.py
// We were unable to find the broadcom channel numbers for these pins, but (as of April
// 2023) Viam doesn't use those values for anything anyway.
var jetsonOrinAGXPins = []genericlinux.PinDefinition{
	{map[int]int{32: 22}, map[int]string{32: "PDD.02"}, "c2f0000.gpio", 3, -1, "I2C4_DAT", "GP16_I2C8_DAT", "", -1},
	{map[int]int{32: 21}, map[int]string{32: "PDD.01"}, "c2f0000.gpio", 5, -1, "I2C4_CLK", "GP81_I2C9_CLK", "", -1},
	{map[int]int{164: 106}, map[int]string{164: "PQ.06"}, "2200000.gpio", 7, 4, "MCLK05", "GP66", "", -1},
	// Output-only (due to hardware limitation)
	{map[int]int{164: 110}, map[int]string{164: "PR.02"}, "2200000.gpio", 8, -1, "UART1_TX", "GP70_UART1_TXD_BOOT2_STRAP", "", -1},
	// Input-only (due to hardware limitation)
	{map[int]int{164: 111}, map[int]string{164: "PR.03"}, "2200000.gpio", 10, -1, "UART1_RX", "GP71_UART1_RXD", "", -1},
	// Output-only (due to hardware limitation)
	{map[int]int{164: 112}, map[int]string{164: "PR.04"}, "2200000.gpio", 11, 17, "UART1_RTS", "GP72_UART1_RTS_N", "", -1},
	{map[int]int{164: 50}, map[int]string{164: "PH.07"}, "2200000.gpio", 12, 18, "I2S2_CLK", "GP122", "", -1},
	{map[int]int{164: 108}, map[int]string{164: "PR.00"}, "2200000.gpio", 13, 27, "PWM01", "GP68", "", -1},
	{map[int]int{164: 85}, map[int]string{164: "PN.01"}, "2200000.gpio", 15, 22, "GPIO27", "GP88_PWM1", "3280000.pwm", 0},
	{map[int]int{32: 9}, map[int]string{32: "PBB.01"}, "c2f0000.gpio", 16, 23, "GPIO08", "GP26", "", -1},
	{map[int]int{164: 43}, map[int]string{164: "PH.00"}, "2200000.gpio", 18, 24, "GPIO35", "GP115", "32c0000.pwm", 0},
	{map[int]int{164: 135}, map[int]string{164: "PZ.05"}, "2200000.gpio", 19, 10, "SPI1_MOSI", "GP49_SPI1_MOSI", "", -1},
	{map[int]int{164: 134}, map[int]string{164: "PZ.04"}, "2200000.gpio", 21, 9, "SPI1_MISO", "GP48_SPI1_MISO", "", -1},
	{map[int]int{164: 96}, map[int]string{164: "PP.04"}, "2200000.gpio", 22, 25, "GPIO17", "GP56", "", -1},
	{map[int]int{164: 133}, map[int]string{164: "PZ.03"}, "2200000.gpio", 23, 11, "SPI1_CLK", "GP47_SPI1_CLK", "", -1},
	{map[int]int{164: 136}, map[int]string{164: "PZ.06"}, "2200000.gpio", 24, 8, "SPI1_CS0_N", "GP50_SPI1_CS0_N", "", -1},
	{map[int]int{164: 137}, map[int]string{164: "PZ.07"}, "2200000.gpio", 26, 7, "SPI1_CS1_N", "GP51_SPI1_CS1_N", "", -1},
	{map[int]int{32: 20}, map[int]string{32: "PDD.00"}, "c2f0000.gpio", 27, -1, "I2C2_DAT", "GP14_I2C2_DAT", "", -1},
	{map[int]int{32: 19}, map[int]string{32: "PCC.07"}, "c2f0000.gpio", 28, -1, "I2C2_CLK", "GP13_I2C2_CLK", "", -1},
	{map[int]int{32: 1}, map[int]string{32: "PAA.01"}, "c2f0000.gpio", 29, 5, "CAN0_DIN", "GP18_CAN0_DIN", "", -1},
	{map[int]int{32: 0}, map[int]string{32: "PAA.00"}, "c2f0000.gpio", 31, 6, "CAN0_DOUT", "GP17_CAN0_DOUT", "", -1},
	{map[int]int{32: 8}, map[int]string{32: "PBB.00"}, "c2f0000.gpio", 32, 12, "GPIO09", "GP25", "", -1},
	{map[int]int{32: 2}, map[int]string{32: "PAA.02"}, "c2f0000.gpio", 33, 13, "CAN1_DOUT", "GP19_CAN1_DOUT", "", -1},
	{map[int]int{164: 53}, map[int]string{164: "PI.02"}, "2200000.gpio", 35, 19, "I2S2_FS", "GP125", "", -1},
	// Input-only (due to hardware limitation)
	{map[int]int{164: 113}, map[int]string{164: "PR.05"}, "2200000.gpio", 36, 16, "UART1_CTS", "GP73_UART1_CTS_N", "", -1},
	{map[int]int{32: 3}, map[int]string{32: "PAA.03"}, "c2f0000.gpio", 37, 26, "CAN1_DIN", "GP20_CAN1_DIN", "", -1},
	{map[int]int{164: 52}, map[int]string{164: "PI.01"}, "2200000.gpio", 38, 20, "I2S2_DIN", "GP124", "", -1},
	{map[int]int{164: 51}, map[int]string{164: "PI.00"}, "2200000.gpio", 40, 21, "I2S2_DOUT", "GP123", "", -1},
}

// This pin mapping is used for both the Jetson Orin NX and the Jetson Orin Nano.
var jetsonOrinNXPins = []genericlinux.PinDefinition{
	{map[int]int{164: 144}, map[int]string{164: "PAC.06"}, "2200000.gpio", 7, 4, "GPIO09", "GP167", "", -1},
	{map[int]int{164: 112}, map[int]string{164: "PR.04"}, "2200000.gpio", 11, 17, "UART1_RTS", "GP72_UART1_RTS_N", "", -1},
	{map[int]int{164: 50}, map[int]string{164: "PH.07"}, "2200000.gpio", 12, 18, "I2S0_SCLK", "GP122", "", -1},
	{map[int]int{164: 122}, map[int]string{164: "PY.00"}, "2200000.gpio", 13, 27, "SPI1_SCK", "GP36_SPI3_CLK", "", -1},
	{map[int]int{164: 85}, map[int]string{164: "PN.01"}, "2200000.gpio", 15, 22, "GPIO12", "GP88_PWM1", "3280000.pwm", 0},
	{map[int]int{164: 126}, map[int]string{164: "PY.04"}, "2200000.gpio", 16, 23, "SPI1_CS1", "GP40_SPI3_CS1_N", "", -1},
	{map[int]int{164: 125}, map[int]string{164: "PY.03"}, "2200000.gpio", 18, 24, "SPI1_CS0", "GP39_SPI3_CS0_N", "", -1},
	{map[int]int{164: 135}, map[int]string{164: "PZ.05"}, "2200000.gpio", 19, 10, "SPI0_MOSI", "GP49_SPI1_MOSI", "", -1},
	{map[int]int{164: 134}, map[int]string{164: "PZ.04"}, "2200000.gpio", 21, 9, "SPI0_MISO", "GP48_SPI1_MISO", "", -1},
	{map[int]int{164: 123}, map[int]string{164: "PY.01"}, "2200000.gpio", 22, 25, "SPI1_MISO", "GP37_SPI3_MISO", "", -1},
	{map[int]int{164: 133}, map[int]string{164: "PZ.03"}, "2200000.gpio", 23, 11, "SPI0_SCK", "GP47_SPI1_CLK", "", -1},
	{map[int]int{164: 136}, map[int]string{164: "PZ.06"}, "2200000.gpio", 24, 8, "SPI0_CS0", "GP50_SPI1_CS0_N", "", -1},
	{map[int]int{164: 137}, map[int]string{164: "PZ.07"}, "2200000.gpio", 26, 7, "SPI0_CS1", "GP51_SPI1_CS1_N", "", -1},
	{map[int]int{164: 105}, map[int]string{164: "PQ.05"}, "2200000.gpio", 29, 5, "GPIO01", "GP65", "", -1},
	{map[int]int{164: 106}, map[int]string{164: "PQ.06"}, "2200000.gpio", 31, 6, "GPIO11", "GP66", "", -1},
	{map[int]int{164: 41}, map[int]string{164: "PG.06"}, "2200000.gpio", 32, 12, "GPIO07", "GP113_PWM7", "", -1},
	{map[int]int{164: 43}, map[int]string{164: "PH.00"}, "2200000.gpio", 33, 13, "GPIO13", "GP115", "32c0000.pwm", 0},
	{map[int]int{164: 53}, map[int]string{164: "PI.02"}, "2200000.gpio", 35, 19, "I2S0_FS", "GP125", "", -1},
	{map[int]int{164: 113}, map[int]string{164: "PR.05"}, "2200000.gpio", 36, 16, "UART1_CTS", "GP73_UART1_CTS_N", "", -1},
	{map[int]int{164: 124}, map[int]string{164: "PY.02"}, "2200000.gpio", 37, 26, "SPI1_MOSI", "GP38_SPI3_MOSI", "", -1},
	{map[int]int{164: 52}, map[int]string{164: "PI.01"}, "2200000.gpio", 38, 20, "I2S0_SDIN", "GP124", "", -1},
	{map[int]int{164: 51}, map[int]string{164: "PI.00"}, "2200000.gpio", 40, 21, "I2S0_SDOUT", "GP123", "", -1},
}

var boardInfoMappings = map[string]genericlinux.BoardInformation{
	claraAGXXavier: {
		claraAGXXavierPins,
		[]string{"nvidia,e3900-0000+p2888-0004"},
	},
	jetsonNX: {
		jetsonNXPins,
		[]string{
			"nvidia,p3509-0000+p3668-0000",
			"nvidia,p3509-0000+p3668-0001",
			"nvidia,p3449-0000+p3668-0000",
			"nvidia,p3449-0000+p3668-0001",
		},
	},
	jetsonXavier: {
		jetsonXavierPins,
		[]string{
			"nvidia,p2972-0000",
			"nvidia,p2972-0006",
			"nvidia,jetson-xavier",
			"nvidia,galen-industrial",
			"nvidia,jetson-xavier-industrial",
		},
	},
	jetsonTX2NX: {
		jetsonTX2NXPins,
		[]string{
			"nvidia,p3509-0000+p3636-0001",
		},
	},
	jetsonTX2: {
		jetsonTX2Pins,
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
		jetsonTX1Pins,
		[]string{
			"nvidia,p2371-2180",
			"nvidia,jetson-cv",
		},
	},
	jetsonNano: {
		jetsonNanoPins,
		[]string{
			"nvidia,p3450-0000",
			"nvidia,p3450-0002",
			"nvidia,jetson-nano",
		},
	},
	jetsonOrinAGX: {
		jetsonOrinAGXPins,
		[]string{
			"nvidia,p3737-0000+p3701-0000",
			"nvidia,p3737-0000+p3701-0004",
		},
	},
	jetsonOrinNX: {
		jetsonOrinNXPins,
		[]string{
			"nvidia,p3509-0000+p3767-0000",
		},
	},
	jetsonOrinNano: {
		jetsonOrinNXPins, // The Jetson Orin Nano has the exact same pinout as the Jetson Orin NX.
		[]string{
			"nvidia,p3768-0000+p3767-0003",
			"nvidia,p3768-0000+p3767-0005",
			"nvidia,p3767-0003",
			"nvidia,p3767-0005",
		},
	},
}

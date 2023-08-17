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

// TODO [RSDK-3596]: fix ngpio numbers in pin definitions for the jetsonTX1, jetsonNano.
var claraAGXXavierPins = []genericlinux.PinDefinition{
	{Name: "7", Ngpio: 169, LineNumber: 106, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", Ngpio: 169, LineNumber: 112, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", Ngpio: 169, LineNumber: 51, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", Ngpio: 169, LineNumber: 96, PwmChipSysfsDir: "", PwmID: -1},
	//  Older versions of L4T don"t enable this PWM controller in DT, so this PWM
	// channel may not be available.
	{Name: "15", Ngpio: 169, LineNumber: 84, PwmChipSysfsDir: "3280000.pwm", PwmID: 0},
	{Name: "16", Ngpio: 30, LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", Ngpio: 169, LineNumber: 44, PwmChipSysfsDir: "32c0000.pwm", PwmID: 0},
	{Name: "19", Ngpio: 169, LineNumber: 162, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", Ngpio: 169, LineNumber: 161, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", Ngpio: 169, LineNumber: 101, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", Ngpio: 169, LineNumber: 160, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", Ngpio: 169, LineNumber: 163, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", Ngpio: 169, LineNumber: 164, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", Ngpio: 30, LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", Ngpio: 30, LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", Ngpio: 30, LineNumber: 9, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "33", Ngpio: 30, LineNumber: 0, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "35", Ngpio: 69, LineNumber: 54, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to base board)
	{Name: "36", Ngpio: 169, LineNumber: 113, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", Ngpio: 30, LineNumber: 1, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", Ngpio: 69, LineNumber: 53, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", Ngpio: 69, LineNumber: 52, PwmChipSysfsDir: "", PwmID: -1},
}

//nolint:dupl // This is not actually a duplicate of jetsonTX2NXPins despite what the linter thinks
var jetsonNXPins = []genericlinux.PinDefinition{
	{Name: "7", Ngpio: 169, LineNumber: 118, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", Ngpio: 169, LineNumber: 112, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", Ngpio: 169, LineNumber: 127, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", Ngpio: 169, LineNumber: 149, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", Ngpio: 30, LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "16", Ngpio: 169, LineNumber: 153, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", Ngpio: 169, LineNumber: 152, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", Ngpio: 169, LineNumber: 162, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", Ngpio: 169, LineNumber: 161, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", Ngpio: 169, LineNumber: 150, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", Ngpio: 169, LineNumber: 160, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", Ngpio: 169, LineNumber: 163, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", Ngpio: 169, LineNumber: 164, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", Ngpio: 169, LineNumber: 105, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", Ngpio: 169, LineNumber: 106, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", Ngpio: 169, LineNumber: 108, PwmChipSysfsDir: "32f0000.pwm", PwmID: 0},
	{Name: "33", Ngpio: 169, LineNumber: 84, PwmChipSysfsDir: "3280000.pwm", PwmID: 0},
	{Name: "35", Ngpio: 169, LineNumber: 130, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "36", Ngpio: 169, LineNumber: 113, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", Ngpio: 169, LineNumber: 151, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", Ngpio: 169, LineNumber: 129, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", Ngpio: 169, LineNumber: 128, PwmChipSysfsDir: "", PwmID: -1},
}

var jetsonXavierPins = []genericlinux.PinDefinition{
	{Name: "7", Ngpio: 169, LineNumber: 106, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", Ngpio: 169, LineNumber: 112, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", Ngpio: 169, LineNumber: 51, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", Ngpio: 169, LineNumber: 108, PwmChipSysfsDir: "32f0000.pwm", PwmID: 0},
	// Older versions of L4T don't enable this PWM controller in DT, so this PWM
	// channel may not be available.
	{Name: "15", Ngpio: 169, LineNumber: 84, PwmChipSysfsDir: "3280000.pwm", PwmID: 0},
	{Name: "16", Ngpio: 30, LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", Ngpio: 169, LineNumber: 44, PwmChipSysfsDir: "32c0000.pwm", PwmID: 0},
	{Name: "19", Ngpio: 169, LineNumber: 162, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", Ngpio: 169, LineNumber: 161, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", Ngpio: 169, LineNumber: 101, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", Ngpio: 169, LineNumber: 160, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", Ngpio: 169, LineNumber: 163, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", Ngpio: 169, LineNumber: 164, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", Ngpio: 30, LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", Ngpio: 30, LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", Ngpio: 30, LineNumber: 9, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "33", Ngpio: 30, LineNumber: 0, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "35", Ngpio: 169, LineNumber: 54, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to base board)
	{Name: "36", Ngpio: 169, LineNumber: 113, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", Ngpio: 30, LineNumber: 1, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", Ngpio: 169, LineNumber: 53, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", Ngpio: 169, LineNumber: 52, PwmChipSysfsDir: "", PwmID: -1},
}

//nolint:dupl // This is not actually a duplicate of jetsonNXPins despite what the linter thinks
var jetsonTX2NXPins = []genericlinux.PinDefinition{
	{Name: "7", Ngpio: 192, LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", Ngpio: 64, LineNumber: 28, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", Ngpio: 192, LineNumber: 72, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", Ngpio: 64, LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", Ngpio: 192, LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "16", Ngpio: 192, LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", Ngpio: 64, LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", Ngpio: 192, LineNumber: 58, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", Ngpio: 192, LineNumber: 57, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", Ngpio: 64, LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", Ngpio: 192, LineNumber: 56, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", Ngpio: 192, LineNumber: 59, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", Ngpio: 192, LineNumber: 163, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", Ngpio: 192, LineNumber: 105, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", Ngpio: 64, LineNumber: 50, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", Ngpio: 64, LineNumber: 8, PwmChipSysfsDir: "3280000.pwm", PwmID: 0},
	{Name: "33", Ngpio: 64, LineNumber: 13, PwmChipSysfsDir: "32a0000.pwm", PwmID: 0},
	{Name: "35", Ngpio: 192, LineNumber: 75, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "36", Ngpio: 64, LineNumber: 29, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", Ngpio: 64, LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", Ngpio: 192, LineNumber: 74, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", Ngpio: 192, LineNumber: 73, PwmChipSysfsDir: "", PwmID: -1},
}

var jetsonTX2Pins = []genericlinux.PinDefinition{
	{Name: "7", DeviceName: "gpiochip0", LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
	// Output-only (due to base board)
	{Name: "11", DeviceName: "gpiochip0", LineNumber: 146, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "gpiochip0", LineNumber: 72, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "gpiochip0", LineNumber: 77, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", DeviceName: "UNKNOWN", LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to module):
	{Name: "16", DeviceName: "gpiochip1", LineNumber: 40, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "gpiochip0", LineNumber: 161, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", DeviceName: "gpiochip0", LineNumber: 109, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "gpiochip0", LineNumber: 108, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", DeviceName: "UNKNOWN", LineNumber: 14, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", DeviceName: "gpiochip0", LineNumber: 107, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", DeviceName: "gpiochip0", LineNumber: 110, PwmChipSysfsDir: "", PwmID: -1},
	// Board pin 26 is not available on this board
	{Name: "29", DeviceName: "gpiochip0", LineNumber: 78, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", DeviceName: "gpiochip1", LineNumber: 42, PwmChipSysfsDir: "", PwmID: -1},
	// Output-only (due to module):
	{Name: "32", DeviceName: "gpiochip1", LineNumber: 41, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "33", DeviceName: "gpiochip0", LineNumber: 69, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "35", DeviceName: "gpiochip0", LineNumber: 75, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to base board) IF NVIDIA debug card NOT plugged in
	// Output-only (due to base board) IF NVIDIA debug card plugged in
	{Name: "36", DeviceName: "gpiochip0", LineNumber: 147, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "gpiochip0", LineNumber: 68, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "gpiochip0", LineNumber: 74, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "gpiochip0", LineNumber: 73, PwmChipSysfsDir: "", PwmID: -1},
}

var jetsonTX1Pins = []genericlinux.PinDefinition{
	{Name: "7", DeviceName: "UNKNOWN", LineNumber: 216, PwmChipSysfsDir: "", PwmID: -1},
	// Output-only (due to base board)
	{Name: "11", DeviceName: "UNKNOWN", LineNumber: 162, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "UNKNOWN", LineNumber: 11, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "UNKNOWN", LineNumber: 38, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", DeviceName: "UNKNOWN", LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "16", DeviceName: "UNKNOWN", LineNumber: 37, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "UNKNOWN", LineNumber: 184, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", DeviceName: "UNKNOWN", LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "UNKNOWN", LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", DeviceName: "UNKNOWN", LineNumber: 14, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", DeviceName: "UNKNOWN", LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", DeviceName: "UNKNOWN", LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", DeviceName: "UNKNOWN", LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", DeviceName: "UNKNOWN", LineNumber: 219, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", DeviceName: "UNKNOWN", LineNumber: 186, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", DeviceName: "UNKNOWN", LineNumber: 36, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "33", DeviceName: "UNKNOWN", LineNumber: 63, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "35", DeviceName: "UNKNOWN", LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to base board) IF NVIDIA debug card NOT plugged in
	// Input-only (due to base board) (always reads fixed value) IF NVIDIA debug card plugged in
	{Name: "36", DeviceName: "UNKNOWN", LineNumber: 163, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "UNKNOWN", LineNumber: 187, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "UNKNOWN", LineNumber: 9, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "UNKNOWN", LineNumber: 10, PwmChipSysfsDir: "", PwmID: -1},
}

var jetsonNanoPins = []genericlinux.PinDefinition{
	{Name: "7", DeviceName: "UNKNOWN", LineNumber: 216, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", DeviceName: "UNKNOWN", LineNumber: 50, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "UNKNOWN", LineNumber: 79, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "UNKNOWN", LineNumber: 14, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", DeviceName: "UNKNOWN", LineNumber: 194, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "16", DeviceName: "UNKNOWN", LineNumber: 232, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "UNKNOWN", LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", DeviceName: "UNKNOWN", LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "UNKNOWN", LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", DeviceName: "UNKNOWN", LineNumber: 13, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", DeviceName: "UNKNOWN", LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", DeviceName: "UNKNOWN", LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", DeviceName: "UNKNOWN", LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", DeviceName: "UNKNOWN", LineNumber: 149, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", DeviceName: "UNKNOWN", LineNumber: 200, PwmChipSysfsDir: "", PwmID: -1},
	// Older versions of L4T have a DT bug which instantiates a bogus device
	// which prevents this library from using this PWM channel.
	{Name: "32", DeviceName: "UNKNOWN", LineNumber: 168, PwmChipSysfsDir: "7000a000.pwm", PwmID: 0},
	{Name: "33", DeviceName: "UNKNOWN", LineNumber: 38, PwmChipSysfsDir: "7000a000.pwm", PwmID: 2},
	{Name: "35", DeviceName: "UNKNOWN", LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "36", DeviceName: "UNKNOWN", LineNumber: 51, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "UNKNOWN", LineNumber: 12, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "UNKNOWN", LineNumber: 77, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "UNKNOWN", LineNumber: 78, PwmChipSysfsDir: "", PwmID: -1},
}

// There are 6 pins whose Broadcom SOC channel is -1 (pins 3, 5, 8, 10, 27, and 28). We
// added these pin definitions ourselves because they're not present in
// https://github.com/NVIDIA/jetson-gpio/blob/master/lib/python/Jetson/GPIO/gpio_pin_data.py
// We were unable to find the broadcom channel numbers for these pins, but (as of April
// 2023) Viam doesn't use those values for anything anyway.
var jetsonOrinAGXPins = []genericlinux.PinDefinition{
	{Name: "3", DeviceName: "gpiochip1", LineNumber: 22, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "5", DeviceName: "gpiochip1", LineNumber: 21, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "7", DeviceName: "gpiochip0", LineNumber: 106, PwmChipSysfsDir: "", PwmID: -1},
	// Output-only (due to hardware limitation)
	{Name: "8", DeviceName: "gpiochip0", LineNumber: 110, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to hardware limitation)
	{Name: "10", DeviceName: "gpiochip0", LineNumber: 111, PwmChipSysfsDir: "", PwmID: -1},
	// Output-only (due to hardware limitation)
	{Name: "11", DeviceName: "gpiochip0", LineNumber: 112, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "gpiochip0", LineNumber: 50, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "gpiochip0", LineNumber: 108, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", DeviceName: "gpiochip0", LineNumber: 85, PwmChipSysfsDir: "3280000.pwm", PwmID: 0},
	{Name: "16", DeviceName: "gpiochip1", LineNumber: 9, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "gpiochip0", LineNumber: 43, PwmChipSysfsDir: "32c0000.pwm", PwmID: 0},
	{Name: "19", DeviceName: "gpiochip0", LineNumber: 135, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "gpiochip0", LineNumber: 134, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", DeviceName: "gpiochip0", LineNumber: 96, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", DeviceName: "gpiochip0", LineNumber: 133, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", DeviceName: "gpiochip0", LineNumber: 136, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", DeviceName: "gpiochip0", LineNumber: 137, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "27", DeviceName: "gpiochip1", LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "28", DeviceName: "gpiochip1", LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", DeviceName: "gpiochip1", LineNumber: 1, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", DeviceName: "gpiochip1", LineNumber: 0, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", DeviceName: "gpiochip1", LineNumber: 8, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "33", DeviceName: "gpiochip1", LineNumber: 2, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "35", DeviceName: "gpiochip0", LineNumber: 53, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to hardware limitation)
	{Name: "36", DeviceName: "gpiochip0", LineNumber: 113, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "gpiochip1", LineNumber: 3, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "gpiochip0", LineNumber: 52, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "gpiochip0", LineNumber: 51, PwmChipSysfsDir: "", PwmID: -1},
}

// This pin mapping is used for both the Jetson Orin NX and the Jetson Orin Nano.
var jetsonOrinNXPins = []genericlinux.PinDefinition{
	{Name: "7", DeviceName: "gpiochip0", LineNumber: 144, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", DeviceName: "gpiochip0", LineNumber: 112, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "gpiochip0", LineNumber: 50, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "gpiochip0", LineNumber: 122, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", DeviceName: "gpiochip0", LineNumber: 85, PwmChipSysfsDir: "3280000.pwm", PwmID: 0},
	{Name: "16", DeviceName: "gpiochip0", LineNumber: 126, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "gpiochip0", LineNumber: 125, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", DeviceName: "gpiochip0", LineNumber: 135, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "gpiochip0", LineNumber: 134, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", DeviceName: "gpiochip0", LineNumber: 123, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", DeviceName: "gpiochip0", LineNumber: 133, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", DeviceName: "gpiochip0", LineNumber: 136, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", DeviceName: "gpiochip0", LineNumber: 137, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", DeviceName: "gpiochip0", LineNumber: 105, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", DeviceName: "gpiochip0", LineNumber: 106, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", DeviceName: "gpiochip0", LineNumber: 41, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "33", DeviceName: "gpiochip0", LineNumber: 43, PwmChipSysfsDir: "32c0000.pwm", PwmID: 0},
	{Name: "35", DeviceName: "gpiochip0", LineNumber: 53, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "36", DeviceName: "gpiochip0", LineNumber: 113, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "gpiochip0", LineNumber: 124, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "gpiochip0", LineNumber: 52, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "gpiochip0", LineNumber: 51, PwmChipSysfsDir: "", PwmID: -1},
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

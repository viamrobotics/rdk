package jetson

import (
	"go.viam.com/rdk/components/board/genericlinux"
)

const (
	jetsonTX2      = "jetson_tx2"
	jetsonNano     = "jetson_nano"
	jetsonTX2NX    = "jetson_tx2_NX"
	jetsonOrinAGX  = "jetson_orin_agx"
	jetsonOrinNX   = "jetson_orin_nx"
	jetsonOrinNano = "jetson_orin_nano"
)

//nolint:dupl // This is not actually a duplicate of jetsonNanoPins despite what the linter thinks
var jetsonTX2NXPins = []genericlinux.PinDefinition{
	{Name: "7", DeviceName: "gpiochip0", LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", DeviceName: "gpiochip1", LineNumber: 28, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "gpiochip0", LineNumber: 72, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "gpiochip1", LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", DeviceName: "gpiochip0", LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "16", DeviceName: "gpiochip0", LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "gpiochip1", LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", DeviceName: "gpiochip0", LineNumber: 58, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "gpiochip0", LineNumber: 57, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", DeviceName: "gpiochip1", LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", DeviceName: "gpiochip0", LineNumber: 56, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", DeviceName: "gpiochip0", LineNumber: 59, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", DeviceName: "gpiochip0", LineNumber: 163, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", DeviceName: "gpiochip0", LineNumber: 105, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", DeviceName: "gpiochip1", LineNumber: 50, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "32", DeviceName: "gpiochip1", LineNumber: 8, PwmChipSysfsDir: "3280000.pwm", PwmID: 0},
	{Name: "33", DeviceName: "gpiochip1", LineNumber: 13, PwmChipSysfsDir: "32a0000.pwm", PwmID: 0},
	{Name: "35", DeviceName: "gpiochip0", LineNumber: 75, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "36", DeviceName: "gpiochip1", LineNumber: 29, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "gpiochip1", LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "gpiochip0", LineNumber: 74, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "gpiochip0", LineNumber: 73, PwmChipSysfsDir: "", PwmID: -1},
}

var jetsonTX2Pins = []genericlinux.PinDefinition{
	{Name: "7", DeviceName: "gpiochip0", LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
	// Output-only (due to base board)
	{Name: "11", DeviceName: "gpiochip0", LineNumber: 146, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "gpiochip0", LineNumber: 72, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "gpiochip0", LineNumber: 77, PwmChipSysfsDir: "", PwmID: -1},
	// TODO[RSDK-6089]: fix this
	{Name: "15", DeviceName: "UNKNOWN", LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
	// Input-only (due to module):
	{Name: "16", DeviceName: "gpiochip1", LineNumber: 40, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "gpiochip0", LineNumber: 161, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", DeviceName: "gpiochip0", LineNumber: 109, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "gpiochip0", LineNumber: 108, PwmChipSysfsDir: "", PwmID: -1},
	// TODO[RSDK-6089]: fix this
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

//nolint:dupl // This is not actually a duplicate of jetsonTX2NXPins despite what the linter thinks
var jetsonNanoPins = []genericlinux.PinDefinition{
	{Name: "7", DeviceName: "gpiochip0", LineNumber: 216, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "11", DeviceName: "gpiochip0", LineNumber: 50, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "12", DeviceName: "gpiochip0", LineNumber: 79, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "13", DeviceName: "gpiochip0", LineNumber: 14, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "15", DeviceName: "gpiochip0", LineNumber: 194, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "16", DeviceName: "gpiochip0", LineNumber: 232, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "18", DeviceName: "gpiochip0", LineNumber: 15, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "19", DeviceName: "gpiochip0", LineNumber: 16, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "21", DeviceName: "gpiochip0", LineNumber: 17, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "22", DeviceName: "gpiochip0", LineNumber: 13, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "23", DeviceName: "gpiochip0", LineNumber: 18, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "24", DeviceName: "gpiochip0", LineNumber: 19, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "26", DeviceName: "gpiochip0", LineNumber: 20, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "29", DeviceName: "gpiochip0", LineNumber: 149, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "31", DeviceName: "gpiochip0", LineNumber: 200, PwmChipSysfsDir: "", PwmID: -1},
	// Older versions of L4T have a DT bug which instantiates a bogus device
	// which prevents this library from using this PWM channel.
	{Name: "32", DeviceName: "gpiochip0", LineNumber: 168, PwmChipSysfsDir: "7000a000.pwm", PwmID: 0},
	{Name: "33", DeviceName: "gpiochip0", LineNumber: 38, PwmChipSysfsDir: "7000a000.pwm", PwmID: 2},
	{Name: "35", DeviceName: "gpiochip0", LineNumber: 76, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "36", DeviceName: "gpiochip0", LineNumber: 51, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "gpiochip0", LineNumber: 12, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "gpiochip0", LineNumber: 77, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "gpiochip0", LineNumber: 78, PwmChipSysfsDir: "", PwmID: -1},
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
	// Pin 32 supposedly has hardware PWM support, but we've been unable to turn it on.
	{Name: "32", DeviceName: "gpiochip0", LineNumber: 41, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "33", DeviceName: "gpiochip0", LineNumber: 43, PwmChipSysfsDir: "32c0000.pwm", PwmID: 0},
	{Name: "35", DeviceName: "gpiochip0", LineNumber: 53, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "36", DeviceName: "gpiochip0", LineNumber: 113, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "37", DeviceName: "gpiochip0", LineNumber: 124, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "38", DeviceName: "gpiochip0", LineNumber: 52, PwmChipSysfsDir: "", PwmID: -1},
	{Name: "40", DeviceName: "gpiochip0", LineNumber: 51, PwmChipSysfsDir: "", PwmID: -1},
}

var boardInfoMappings = map[string]genericlinux.BoardInformation{
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

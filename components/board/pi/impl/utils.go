//go:build linux && (arm64 || arm) && !no_pigpio

package piimpl

import "fmt"

// piHWPinToBroadcom maps the hardware inscribed pin number to
// its Broadcom pin. For the sake of programming, a user typically
// knows the hardware pin since they have the board on hand but does
// not know the corresponding Broadcom pin.
var piHWPinToBroadcom = map[string]uint{
	// 1 -> 3v3
	// 2 -> 5v
	"3":   2,
	"sda": 2,
	// 4 -> 5v
	"5":   3,
	"scl": 3,
	// 6 -> GND
	"7": 4,
	"8": 14,
	// 9 -> GND
	"10":  15,
	"11":  17,
	"12":  18,
	"clk": 18,
	"13":  27,
	// 14 -> GND
	"15": 22,
	"16": 23,
	// 17 -> 3v3
	"18":   24,
	"19":   10,
	"mosi": 10,
	// 20 -> GND
	"21":   9,
	"miso": 9,
	"22":   25,
	"23":   11,
	"sclk": 11,
	"24":   8,
	"ce0":  8,
	// 25 -> GND
	"26":  7,
	"ce1": 7,
	"27":  0,
	"28":  1,
	"29":  5,
	// 30 -> GND
	"31": 6,
	"32": 12,
	"33": 13,
	// 34 -> GND
	"35": 19,
	"36": 16,
	"37": 26,
	"38": 20,
	// 39 -> GND
	"40": 21,
}

// TODO: we should agree on one config standard for pin definitions
// instead of doing this. Maybe just use the actual pin number?
// It might be reasonable to force users to look up the associations
// online - GV

// broadcomPinFromHardwareLabel returns a Raspberry Pi pin number given
// a hardware label for the pin passed from a config.
func broadcomPinFromHardwareLabel(hwPin string) (uint, bool) {
	pin, ok := piHWPinToBroadcom[hwPin]
	if ok {
		return pin, true
	}
	for _, existingVal := range piHWPinToBroadcom {
		if hwPin == fmt.Sprintf("io%d", existingVal) {
			return existingVal, true
		}
	}
	return 1000, false
}

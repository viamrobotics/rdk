//go:build linux
// +build linux

package gamepad

import (
	"strings"

	"github.com/viamrobotics/evdev"

	"go.viam.com/rdk/components/input"
)

const stadiaPrefix = "Stadia"

// gamepadMappings contains all the axes/button translations for each model.
// use evtest on linux figure out what maps to what.
var gamepadMappings = map[string]Mapping{
	// 8BitDo Pro 2 Wireless, S-input mode
	// Also the Nintendo Switch Pro Controller
	"Pro Controller": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			3:  input.AbsoluteRX,
			4:  input.AbsoluteRY,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			304: input.ButtonSouth,
			305: input.ButtonEast,
			306: input.ButtonWest,
			307: input.ButtonNorth,
			308: input.ButtonLT,
			309: input.ButtonRT,
			310: input.ButtonLT2,
			311: input.ButtonRT2,
			312: input.ButtonSelect,
			313: input.ButtonStart,
			314: input.ButtonLThumb,
			315: input.ButtonRThumb,
			316: input.ButtonMenu,
			317: input.ButtonRecord,
		},
	},
	// Wireless, X-input mode
	"8BitDo Pro 2": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			2:  input.AbsoluteZ,
			3:  input.AbsoluteRX,
			4:  input.AbsoluteRY,
			5:  input.AbsoluteRZ,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			304: input.ButtonSouth,
			305: input.ButtonEast,
			306: input.ButtonWest,
			307: input.ButtonNorth,
			308: input.ButtonLT,
			309: input.ButtonRT,
			310: input.ButtonSelect,
			311: input.ButtonStart,
			312: input.ButtonLThumb,
			313: input.ButtonRThumb,
			139: input.ButtonMenu,
		},
	},
	// Wired, X-input mode 8BitDo Pro 2
	"Microsoft X-Box 360 pad": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			2:  input.AbsoluteZ,
			3:  input.AbsoluteRX,
			4:  input.AbsoluteRY,
			5:  input.AbsoluteRZ,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			304: input.ButtonSouth,
			305: input.ButtonEast,
			307: input.ButtonWest,
			308: input.ButtonNorth,
			310: input.ButtonLT,
			311: input.ButtonRT,
			314: input.ButtonSelect,
			315: input.ButtonStart,
			317: input.ButtonLThumb,
			318: input.ButtonRThumb,
			316: input.ButtonMenu,
		},
	},
	// Xbox Series X|S, wireless mode
	"Xbox Wireless Controller": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			10: input.AbsoluteZ,
			2:  input.AbsoluteRX,
			5:  input.AbsoluteRY,
			9:  input.AbsoluteRZ,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			304: input.ButtonSouth,
			305: input.ButtonEast,
			307: input.ButtonWest,
			308: input.ButtonNorth,
			310: input.ButtonLT,
			311: input.ButtonRT,
			314: input.ButtonSelect,
			315: input.ButtonStart,
			317: input.ButtonLThumb,
			318: input.ButtonRThumb,
			316: input.ButtonMenu,
			167: input.ButtonRecord,
		},
	},
	// Xbox Series X|S, wired mode
	"Microsoft Xbox One X pad": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			2:  input.AbsoluteZ,
			3:  input.AbsoluteRX,
			4:  input.AbsoluteRY,
			5:  input.AbsoluteRZ,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			304: input.ButtonSouth,
			305: input.ButtonEast,
			307: input.ButtonWest,
			308: input.ButtonNorth,
			310: input.ButtonLT,
			311: input.ButtonRT,
			314: input.ButtonSelect,
			315: input.ButtonStart,
			317: input.ButtonLThumb,
			318: input.ButtonRThumb,
			316: input.ButtonMenu,
		},
	},
	// Wireless industrial controller
	"FORT Robotics nVSC Application": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			2:  input.AbsoluteZ,
			3:  input.AbsoluteRX,
			4:  input.AbsoluteRY,
			5:  input.AbsoluteRZ,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			288: input.ButtonSouth,
			289: input.ButtonEast,
			291: input.ButtonWest,
			290: input.ButtonNorth,
			292: input.ButtonEStop,
		},
	},

	// https://www.amazon.com/SQDeal-Joystick-Controller-Vibration-Feedback/dp/B01GR9ZZTS
	"USB Gamepad": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			2:  input.AbsoluteRY,
			5:  input.AbsoluteRX,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			288: input.ButtonNorth,
			289: input.ButtonEast,
			291: input.ButtonWest,
			290: input.ButtonSouth,

			292: input.ButtonLT,
			293: input.ButtonRT,
			294: input.ButtonLT2,
			295: input.ButtonRT2,

			296: input.ButtonSelect,
			297: input.ButtonStart,
		},
	},
	// PS4 Controller
	"Wireless Controller": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			2:  input.AbsoluteZ,
			3:  input.AbsoluteRX,
			4:  input.AbsoluteRY,
			5:  input.AbsoluteRZ,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			304: input.ButtonSouth,
			305: input.ButtonEast,
			307: input.ButtonNorth,
			308: input.ButtonWest,
			310: input.ButtonLT,
			311: input.ButtonRT,
			314: input.ButtonSelect,
			315: input.ButtonStart,
			317: input.ButtonLThumb,
			318: input.ButtonRThumb,
			316: input.ButtonMenu,
		},
	},
	// Logitech G920/G29 Wheel
	"Logitech G920 Driving Force Racing Wheel": {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsolutePedalAccelerator,
			2:  input.AbsolutePedalBrake,
			5:  input.AbsolutePedalClutch,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			288: input.ButtonSouth,
			289: input.ButtonEast,
			291: input.ButtonNorth,
			290: input.ButtonWest,
			293: input.ButtonLT,
			292: input.ButtonRT,
			295: input.ButtonSelect,
			294: input.ButtonStart,
			297: input.ButtonLThumb,
			296: input.ButtonRThumb,
			298: input.ButtonMenu,
		},
	},
	// Stadia Controller
	stadiaPrefix: {
		Axes: map[evdev.AbsoluteType]input.Control{
			0:  input.AbsoluteX,
			1:  input.AbsoluteY,
			10: input.AbsoluteZ,
			2:  input.AbsoluteRX,
			5:  input.AbsoluteRY,
			9:  input.AbsoluteRZ,
			16: input.AbsoluteHat0X,
			17: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.Control{
			304: input.ButtonSouth,
			305: input.ButtonEast,
			307: input.ButtonWest,
			308: input.ButtonNorth,
			310: input.ButtonLT,
			311: input.ButtonRT,
			314: input.ButtonSelect,
			315: input.ButtonStart,
			316: input.ButtonMenu,
			317: input.ButtonLThumb,
			318: input.ButtonRThumb,
		},
	},
}

// MappingForModel returns the mapping for a given model.
func MappingForModel(model string) (Mapping, bool) {
	// Stadia controller device names are unique of the form "StadiaXXXX-XXXX"
	if strings.HasPrefix(model, stadiaPrefix) {
		model = stadiaPrefix
	}
	mapping, ok := gamepadMappings[model]
	return mapping, ok
}

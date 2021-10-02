package gamepad

import (
		"go.viam.com/core/input"
		"github.com/kenshaw/evdev"
)


// GamepadModels contains all the axes/button translations for each model
var GamepadModels = map[string]Mapping{
	"8BitDo Pro 2": Mapping{
		Axes: map[evdev.AbsoluteType]input.ControlCode{
			0: input.AbsoluteX,
			1: input.AbsoluteY,
			2: input.AbsoluteZ,
			3: input.AbsoluteRX,
			4: input.AbsoluteRY,
			5: input.AbsoluteRZ,
			6: input.AbsoluteHat0X,
			7: input.AbsoluteHat0Y,
		},
		Buttons: map[evdev.KeyType]input.ControlCode{
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
}

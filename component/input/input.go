// Package input provides human input, such as buttons, switches, knobs, gamepads, joysticks, keyboards, mice, etc.
package input

import (
	"context"
	"time"
)

// Controller is a logical "container" more than an actual device
// Could be a single gamepad, or a collection of digitalInterrupts and analogReaders, a keyboard, etc.
type Controller interface {
	// Controls returns a list of Controls provided by the Controller
	Controls(ctx context.Context) ([]Control, error)

	// LastEvent returns most recent Event for each input (which should be the current state)
	LastEvents(ctx context.Context) (map[Control]Event, error)

	// RegisterCallback registers a callback that will fire on given EventTypes for a given Control
	RegisterControlCallback(ctx context.Context, control Control, triggers []EventType, ctrlFunc ControlFunction) error
}

// ControlFunction is a callback passed to RegisterControlCallback
type ControlFunction func(ctx context.Context, ev Event)

// EventType represents the type of input event, and is returned by LastEvent() or passed to ControlFunction callbacks.
type EventType string

// EventType list, to be expanded as new input devices are developed
const (
	AllEvents         EventType = "AllEvents"         // Callbacks registered for this event will be called in ADDITION to other registered event callbacks
	Connect           EventType = "Connect"           // Sent at controller initialization, and on reconnects
	Disconnect        EventType = "Disconnect"        // If unplugged, or wireless/network times out
	ButtonPress       EventType = "ButtonPress"       // Typical keypress
	ButtonRelease     EventType = "ButtonRelease"     // Key release
	ButtonChange      EventType = "ButtonChange"      // Both up and down for convinence during registration, not typically emitted
	PositionChangeAbs EventType = "PositionChangeAbs" // Absolute position is reported via Value, a la joysticks
	PositionChangeRel EventType = "PositionChangeRel" // Relative position is reported via Value, a la mice, or simulating axes with up/down buttons
)

// Control identifies the input (specific Axis or Button) of a controller
type Control string

// Controls, to be expanded as new input devices are developed
const (
	// Axes
	AbsoluteX     Control = "AbsoluteX"
	AbsoluteY     Control = "AbsoluteY"
	AbsoluteZ     Control = "AbsoluteZ"
	AbsoluteRX    Control = "AbsoluteRX"
	AbsoluteRY    Control = "AbsoluteRY"
	AbsoluteRZ    Control = "AbsoluteRZ"
	AbsoluteHat0X Control = "AbsoluteHat0X"
	AbsoluteHat0Y Control = "AbsoluteHat0Y"

	// Buttons
	ButtonSouth  Control = "ButtonSouth"
	ButtonEast   Control = "ButtonEast"
	ButtonWest   Control = "ButtonWest"
	ButtonNorth  Control = "ButtonNorth"
	ButtonLT     Control = "ButtonLT"
	ButtonRT     Control = "ButtonRT"
	ButtonLThumb Control = "ButtonLThumb"
	ButtonRThumb Control = "ButtonRThumb"
	ButtonSelect Control = "ButtonSelect"
	ButtonStart  Control = "ButtonStart"
	ButtonMenu   Control = "ButtonMenu"
	ButtonRecord Control = "ButtonRecord"
	ButtonEStop  Control = "ButtonEStop"
)

// Event is passed to the registered ControlFunction or returned by State()
type Event struct {
	Time    time.Time
	Event   EventType
	Control Control // Key or Axis
	Value   float64 // 0 or 1 for buttons, -1.0 to +1.0 for axes
}

// Injectable is used by the WebGamepad interface to inject events
type Injectable interface {
	// InjectEvent allows directly sending an Event (such as a button press) from external code
	InjectEvent(ctx context.Context, event Event) error
}

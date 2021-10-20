// Package input provides human input, such as buttons, switches, knobs, gamepads, joysticks, keyboards, mice, etc.
package input

import (
	"context"
	"time"
)

// Controller is a logical "container" more than an actual device
// Could be a single gamepad, or a collection of digitalInterrupts and analogReaders, a keyboard, etc.
type Controller interface {
	Inputs(ctx context.Context) (map[ControlCode]Input, error)
}

// Input represents a single axis or button, defined by its ControlCode
type Input interface {
	// Name returns the ControlCode
	Name(ctx context.Context) ControlCode

	// LastEvent returns most recent Event (which should be the current state)
	LastEvent(ctx context.Context) (Event, error)

	// RegisterControl registers a callback for the trigger EventType
	RegisterControl(ctx context.Context, ctrlFunc ControlFunction, trigger EventType) error
}

// ControlFunction is a callback passed to RegisterControl
type ControlFunction func(ctx context.Context, inp Input, ev Event)

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

// ControlCode identifies the input (specific Axis or Button) of a controller
type ControlCode string

// ControlCodes, to be expanded as new input devices are developed
const (
	// Axes
	AbsoluteX     ControlCode = "AbsoluteX"
	AbsoluteY     ControlCode = "AbsoluteY"
	AbsoluteZ     ControlCode = "AbsoluteZ"
	AbsoluteRX    ControlCode = "AbsoluteRX"
	AbsoluteRY    ControlCode = "AbsoluteRY"
	AbsoluteRZ    ControlCode = "AbsoluteRZ"
	AbsoluteHat0X ControlCode = "AbsoluteHat0X"
	AbsoluteHat0Y ControlCode = "AbsoluteHat0Y"

	// Buttons
	ButtonSouth  ControlCode = "ButtonSouth"
	ButtonEast   ControlCode = "ButtonEast"
	ButtonWest   ControlCode = "ButtonWest"
	ButtonNorth  ControlCode = "ButtonNorth"
	ButtonLT     ControlCode = "ButtonLT"
	ButtonRT     ControlCode = "ButtonRT"
	ButtonLThumb ControlCode = "ButtonLThumb"
	ButtonRThumb ControlCode = "ButtonRThumb"
	ButtonSelect ControlCode = "ButtonSelect"
	ButtonStart  ControlCode = "ButtonStart"
	ButtonMenu   ControlCode = "ButtonMenu"
	ButtonRecord ControlCode = "ButtonRecord"
)

// Event is passed to the registered ControlFunction or returned by State()
type Event struct {
	Time  time.Time
	Event EventType
	Code  ControlCode // Key or Axis code
	Value float64     // 0 or 1 for buttons, -1.0 to +1.0 for axes
}

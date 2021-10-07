//go:generate stringer -output=input_strings.go -type=EventType,ControlCode

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
	Name(ctx context.Context) string          // The stringified representation of the ControlCode
	State(ctx context.Context) (Event, error) // returns most recent Event (which should be the current state)
	RegisterControl(ctx context.Context, ctrlFunc ControlFunction, trigger EventType) error
}

// ControlFunction is a callback passed to RegisterControl
type ControlFunction func(ctx context.Context, input Input, event Event)

// EventType represents the type of input event, and is returned by State() or passed to ControlFunction callbacks.
// Extensible for further events
// Ex: LongPress might be useful on remote network connections where ButtonDown/ButtonUp timing might be too variable
type EventType uint8

// EventType codes, to be expanded as new input devices are developed
const (
	AllEvents         EventType = 0 // Callbacks registered for this event will be called in ADDITION to other registered event callbacks
	Connect           EventType = 1 // Sent at controller initialization, and on reconnects
	Disconnect        EventType = 2 // If unplugged, or wireless/network times out
	ButtonDown        EventType = 3 // Typical keypress
	ButtonUp          EventType = 4 // Key release
	ButtonChange      EventType = 5 // Both up and down for convinence during registration, not typically emitted
	PositionChangeAbs EventType = 6 // Absolute position is reported via Value, a la joysticks
	PositionChangeRel EventType = 7 // Relative position is reported via Value, a la mice, or simulating axes with up/down buttons
)

// ControlCode identifies the type of input (specific Axis or Button)
// This is similar to KeyCodes used in various HID layers
type ControlCode uint32

// ControlCodes, to be expanded as new input devices are developed
const (
	// Reserving keys under 1000 for overlap with standard keycodes
	// Axes
	AbsoluteX     ControlCode = 1000
	AbsoluteY     ControlCode = 1001
	AbsoluteZ     ControlCode = 1002
	AbsoluteRX    ControlCode = 1003
	AbsoluteRY    ControlCode = 1004
	AbsoluteRZ    ControlCode = 1005
	AbsoluteHat0X ControlCode = 1006
	AbsoluteHat0Y ControlCode = 1007

	// Buttons
	ButtonSouth  ControlCode = 2000
	ButtonEast   ControlCode = 2001
	ButtonWest   ControlCode = 2002
	ButtonNorth  ControlCode = 2003
	ButtonLT     ControlCode = 2004
	ButtonRT     ControlCode = 2005
	ButtonLThumb ControlCode = 2006
	ButtonRThumb ControlCode = 2007
	ButtonSelect ControlCode = 2008
	ButtonStart  ControlCode = 2009
	ButtonMenu   ControlCode = 2010
	ButtonRecord ControlCode = 2011
)

// Event is passed to the registered ControlFunction or returned by State()
type Event struct {
	Time  time.Time
	Event EventType
	Code  ControlCode // Key or Axis code
	Value float64     // 0 or 1 for buttons, -1.0 to +1.0 for axes
}

package input

import (
	"context"
	"time"
)


// This is a logical "container" more than an actual device
// Could be a single gamepad, or a collection of digitalInterrupts and analogReaders, some local, some remote
type Controller interface {
	Inputs(ctx context.Context) (map[ControlCode]Input, error)
	EventDispatcher(ctx context.Context) 
}

type Input interface {
	Name(ctx context.Context) string
	State(ctx context.Context) (Event, error) // returns most recent event (which should be the current state)
	RegisterControl(ctx context.Context, ctrlFunc ControlFunction, trigger EventType) error
}

type ControlFunction func(ctx context.Context, input Input, event Event) error

// Extensible for further events
// Ex: LongPress might be useful on remote network connections where ButtonDown/ButtonUp timing might be too variable
type EventType uint8
const (
	All EventType = 0
	Connect EventType = 1
	Disconnect EventType = 2 // If unplugged, or wireless/network times out
	ButtonDown EventType = 3
	ButtonUp EventType = 4
	ButtonChange EventType = 5 // Both up and down for convinence
	PositionChangeAbs EventType = 6 // Absolute position is reported, a la joysticks
	PositionChangeRel EventType = 7 // Relative position is reported, a la mice, or simulating axes with up/down buttons
)

// Effectively keycode mappings
// This allows one InputButton to provide multiple keys via codes, so a keyboard doesn't need to define 104 keys
// Can be completely ignored for basic gamepad/arrows/select type use as well.
type ControlCode uint32
const (
	// Reserving keys under 1000 for overlap with standard keycodes
	// Axes
	AbsoluteX             ControlCode = 1000
	AbsoluteY             ControlCode = 1001
	AbsoluteZ             ControlCode = 1002
	AbsoluteRX            ControlCode = 1003
	AbsoluteRY            ControlCode = 1004
	AbsoluteRZ            ControlCode = 1005
	AbsoluteHat0X         ControlCode = 1006
	AbsoluteHat0Y         ControlCode = 1007

	// Buttons
	ButtonSouth ControlCode = 2000
	ButtonEast ControlCode = 2001
	ButtonWest ControlCode = 2002
	ButtonNorth ControlCode = 2003
	ButtonLT ControlCode = 2004
	ButtonRT ControlCode = 2005
	ButtonLThumb ControlCode = 2006
	ButtonRThumb ControlCode = 2007
	ButtonSelect ControlCode = 2008
	ButtonStart ControlCode = 2009
	ButtonMenu ControlCode = 2010
)

// The event returned to the registered ControlFunction
type Event struct {
	Time time.Time
	Event EventType
	Code ControlCode // Key or Axis code
	Value float64 // 0 or 1 for buttons, -1.0 to +1.0 for axes
}

// Package input provides human input, such as buttons, switches, knobs, gamepads, joysticks, keyboards, mice, etc.
// For more information, see the [input controller component docs].
//
// [input controller component docs]: https://docs.viam.com/components/input-controller/
package input

import (
	"context"
	"time"

	pb "go.viam.com/api/component/inputcontroller/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Controller]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterInputControllerServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.InputControllerService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource API string input.
const SubtypeName = "input_controller"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named input's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Controller is a logical "container" more than an actual device.
// It could be a single gamepad, or a collection of digitalInterrupts and analogReaders, a keyboard, etc.
// For more information, see the [input controller component docs].
//
// Controls example:
//
//	myController, err := input.FromRobot(machine, "my_input_controller")
//
//	// Get the list of Controls provided by the controller.
//	controls, err := myController.Controls(context.Background(), nil)
//
// For more information, see the [Controls method docs].
//
// Events example:
//
//	myController, err := input.FromRobot(machine, "my_input_controller")
//
//	// Get the most recent Event for each Control.
//	recent_events, err := myController.Events(context.Background(), nil)
//
// For more information, see the [Events method docs].
//
// RegisterControlCallback example:
//
//	// Define a function to handle pressing the Start Menu button, "ButtonStart", on your controller and logging the start time
//	printStartTime := func(ctx context.Context, event input.Event) {
//	    logger.Info("Start Menu Button was pressed at this time: %v", event.Time)
//	}
//
//	myController, err := input.FromRobot(machine, "my_input_controller")
//
//	// Define the EventType "ButtonPress" to serve as the trigger for printStartTime.
//	triggers := []input.EventType{input.ButtonPress}
//
//	// Get the controller's Controls.
//	controls, err := myController.Controls(context.Background(), nil)
//
//	// If the "ButtonStart" Control is found, trigger printStartTime when on "ButtonStart" the event "ButtonPress" occurs.
//	if !slices.Contains(controls, input.ButtonStart) {
//	    logger.Error("button 'ButtonStart' not found; controller may be disconnected")
//	    return
//	}
//
//	myController.RegisterControlCallback(context.Background(), input.ButtonStart, triggers, printStartTime, nil)
//
// For more information, see the [RegisterControlCallback method docs].
//
// [input controller component docs]: https://docs.viam.com/dev/reference/apis/components/input-controller/
// [Controls method docs]: https://docs.viam.com/dev/reference/apis/components/input-controller/#getcontrols
// [Events method docs]: https://docs.viam.com/dev/reference/apis/components/input-controller/#getevents
// [RegisterControlCallback method docs]: https://docs.viam.com/dev/reference/apis/components/input-controller/#registercontrolcallback
type Controller interface {
	resource.Resource

	// Controls returns a list of Controls provided by the Controller
	Controls(ctx context.Context, extra map[string]interface{}) ([]Control, error)

	// Events returns most recent Event for each input (which should be the current state)
	Events(ctx context.Context, extra map[string]interface{}) (map[Control]Event, error)

	// RegisterCallback registers a callback that will fire on given EventTypes for a given Control.
	// The callback is called on the same goroutine as the firer and if any long operation is to occur,
	// the callback should start a goroutine.
	RegisterControlCallback(
		ctx context.Context,
		control Control,
		triggers []EventType,
		ctrlFunc ControlFunction,
		extra map[string]interface{},
	) error
}

// ControlFunction is a callback passed to RegisterControlCallback.
type ControlFunction func(ctx context.Context, ev Event)

// EventType represents the type of input event, and is returned by LastEvent() or passed to ControlFunction callbacks.
type EventType string

// EventType list, to be expanded as new input devices are developed.
const (
	// Callbacks registered for this event will be called in ADDITION to other registered event callbacks.
	AllEvents EventType = "AllEvents"
	// Sent at controller initialization, and on reconnects.
	Connect EventType = "Connect"
	// If unplugged, or wireless/network times out.
	Disconnect EventType = "Disconnect"
	// Typical key press.
	ButtonPress EventType = "ButtonPress"
	// Key release.
	ButtonRelease EventType = "ButtonRelease"
	// Key is held down. This will likely be a repeated event.
	ButtonHold EventType = "ButtonHold"
	// Both up and down for convenience during registration, not typically emitted.
	ButtonChange EventType = "ButtonChange"
	// Absolute position is reported via Value, a la joysticks.
	PositionChangeAbs EventType = "PositionChangeAbs"
	// Relative position is reported via Value, a la mice, or simulating axes with up/down buttons.
	PositionChangeRel EventType = "PositionChangeRel"
)

// Control identifies the input (specific Axis or Button) of a controller.
type Control string

// Controls, to be expanded as new input devices are developed.
const (
	// Axes.
	AbsoluteX     Control = "AbsoluteX"
	AbsoluteY     Control = "AbsoluteY"
	AbsoluteZ     Control = "AbsoluteZ"
	AbsoluteRX    Control = "AbsoluteRX"
	AbsoluteRY    Control = "AbsoluteRY"
	AbsoluteRZ    Control = "AbsoluteRZ"
	AbsoluteHat0X Control = "AbsoluteHat0X"
	AbsoluteHat0Y Control = "AbsoluteHat0Y"

	// Buttons.
	ButtonSouth  Control = "ButtonSouth"
	ButtonEast   Control = "ButtonEast"
	ButtonWest   Control = "ButtonWest"
	ButtonNorth  Control = "ButtonNorth"
	ButtonLT     Control = "ButtonLT"
	ButtonRT     Control = "ButtonRT"
	ButtonLT2    Control = "ButtonLT2"
	ButtonRT2    Control = "ButtonRT2"
	ButtonLThumb Control = "ButtonLThumb"
	ButtonRThumb Control = "ButtonRThumb"
	ButtonSelect Control = "ButtonSelect"
	ButtonStart  Control = "ButtonStart"
	ButtonMenu   Control = "ButtonMenu"
	ButtonRecord Control = "ButtonRecord"
	ButtonEStop  Control = "ButtonEStop"

	// Pedals.
	AbsolutePedalAccelerator Control = "AbsolutePedalAccelerator"
	AbsolutePedalBrake       Control = "AbsolutePedalBrake"
	AbsolutePedalClutch      Control = "AbsolutePedalClutch"
)

// Event is passed to the registered ControlFunction or returned by State().
type Event struct {
	Time    time.Time
	Event   EventType
	Control Control // Key or Axis
	Value   float64 // 0 or 1 for buttons, -1.0 to +1.0 for axes
}

// Triggerable is used by the WebGamepad interface to inject events.
type Triggerable interface {
	// TriggerEvent allows directly sending an Event (such as a button press) from external code
	TriggerEvent(ctx context.Context, event Event, extra map[string]interface{}) error
}

// FromDependencies is a helper for getting the named input controller from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Controller, error) {
	return resource.FromDependencies[Controller](deps, Named(name))
}

// FromRobot is a helper for getting the named input controller from the given Robot.
func FromRobot(r robot.Robot, name string) (Controller, error) {
	return robot.ResourceFromRobot[Controller](r, Named(name))
}

// NamesFromRobot is a helper for getting all input controller names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

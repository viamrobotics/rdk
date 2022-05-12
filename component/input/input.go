// Package input provides human input, such as buttons, switches, knobs, gamepads, joysticks, keyboards, mice, etc.
package input

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/component/generic"
	pb "go.viam.com/rdk/proto/api/component/inputcontroller/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		Status: func(ctx context.Context, resource interface{}) (interface{}, error) {
			return CreateStatus(ctx, resource)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.InputControllerService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterInputControllerServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
}

// SubtypeName is a constant that identifies the component resource subtype string input.
const SubtypeName = resource.SubtypeName("input_controller")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named input's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// Controller is a logical "container" more than an actual device
// Could be a single gamepad, or a collection of digitalInterrupts and analogReaders, a keyboard, etc.
type Controller interface {
	// GetControls returns a list of GetControls provided by the Controller
	GetControls(ctx context.Context) ([]Control, error)

	// LastEvent returns most recent Event for each input (which should be the current state)
	GetEvents(ctx context.Context) (map[Control]Event, error)

	// RegisterCallback registers a callback that will fire on given EventTypes for a given Control
	RegisterControlCallback(ctx context.Context, control Control, triggers []EventType, ctrlFunc ControlFunction) error

	generic.Generic
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
	// Key release.
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
	TriggerEvent(ctx context.Context, event Event) error
}

// WrapWithReconfigurable wraps a Controller with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	c, ok := r.(Controller)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Controller", r)
	}
	if reconfigurable, ok := c.(*reconfigurableInputController); ok {
		return reconfigurable, nil
	}
	return &reconfigurableInputController{actual: c}, nil
}

var (
	_ = Controller(&reconfigurableInputController{})
	_ = resource.Reconfigurable(&reconfigurableInputController{})
)

// FromRobot is a helper for getting the named input controller from the given Robot.
func FromRobot(r robot.Robot, name string) (Controller, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Controller)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("input.Controller", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all input controller names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the input controller.
func CreateStatus(ctx context.Context, resource interface{}) (*pb.Status, error) {
	controller, ok := resource.(Controller)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("input.Controller", resource)
	}
	eventsIn, err := controller.GetEvents(ctx)
	if err != nil {
		return nil, err
	}
	events := make([]*pb.Event, 0, len(eventsIn))
	for _, eventIn := range eventsIn {
		events = append(events, &pb.Event{
			Time:    timestamppb.New(eventIn.Time),
			Event:   string(eventIn.Event),
			Control: string(eventIn.Control),
			Value:   eventIn.Value,
		})
	}

	return &pb.Status{Events: events}, nil
}

type reconfigurableInputController struct {
	mu     sync.RWMutex
	actual Controller
}

func (c *reconfigurableInputController) ProxyFor() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual
}

func (c *reconfigurableInputController) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.Do(ctx, cmd)
}

func (c *reconfigurableInputController) GetControls(ctx context.Context) ([]Control, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.GetControls(ctx)
}

func (c *reconfigurableInputController) GetEvents(ctx context.Context) (map[Control]Event, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.GetEvents(ctx)
}

// TriggerEvent allows directly sending an Event (such as a button press) from external code.
func (c *reconfigurableInputController) TriggerEvent(ctx context.Context, event Event) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	iActual, ok := c.actual.(Triggerable)
	if !ok {
		return errors.New("controller is not Triggerable")
	}
	return iActual.TriggerEvent(ctx, event)
}

func (c *reconfigurableInputController) RegisterControlCallback(
	ctx context.Context,
	control Control,
	triggers []EventType,
	ctrlFunc ControlFunction,
) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actual.RegisterControlCallback(ctx, control, triggers, ctrlFunc)
}

func (c *reconfigurableInputController) Close(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return viamutils.TryClose(ctx, c.actual)
}

// Reconfigure reconfigures the resource.
func (c *reconfigurableInputController) Reconfigure(ctx context.Context, newController resource.Reconfigurable) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	actual, ok := newController.(*reconfigurableInputController)
	if !ok {
		return utils.NewUnexpectedTypeError(c, newController)
	}
	if err := viamutils.TryClose(ctx, c.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	c.actual = actual.actual
	return nil
}

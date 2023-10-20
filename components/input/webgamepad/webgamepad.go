// Package webgamepad implements a web based input controller.
package webgamepad

import (
	"context"
	"sync"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// NOTE: Component NAME (in config file) must be set to "WebGamepad" exactly
// This is because there's no way to get a component's model from a robot.Robot.
var model = resource.DefaultModelFamily.WithModel("webgamepad")

func init() {
	resource.RegisterComponent(input.API, model, resource.Registration[input.Controller, resource.NoNativeConfig]{
		Constructor: NewController,
	})
}

// NewController creates a new gamepad.
func NewController(
	ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.ZapCompatibleLogger,
) (input.Controller, error) {
	return &webGamepad{
		Named:      conf.ResourceName().AsNamed(),
		callbacks:  map[input.Control]map[input.EventType]input.ControlFunction{},
		lastEvents: map[input.Control]input.Event{},
		controls: []input.Control{
			input.AbsoluteX, input.AbsoluteY, input.AbsoluteRX, input.AbsoluteRY,
			input.AbsoluteZ, input.AbsoluteRZ, input.AbsoluteHat0X, input.AbsoluteHat0Y,
			input.ButtonSouth, input.ButtonEast, input.ButtonWest, input.ButtonNorth,
			input.ButtonLT, input.ButtonRT, input.ButtonLThumb, input.ButtonRThumb,
			input.ButtonSelect, input.ButtonStart, input.ButtonMenu,
		},
		logger: logging.FromZapCompatible(logger),
	}, nil
}

// webGamepad is an input.Controller.
type webGamepad struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	controls   []input.Control
	lastEvents map[input.Control]input.Event
	mu         sync.RWMutex
	callbacks  map[input.Control]map[input.EventType]input.ControlFunction
	logger     logging.Logger
}

func (w *webGamepad) makeCallbacks(ctx context.Context, eventOut input.Event) {
	w.mu.Lock()
	w.lastEvents[eventOut.Control] = eventOut
	w.mu.Unlock()

	w.mu.RLock()
	_, ok := w.callbacks[eventOut.Control]
	w.mu.RUnlock()
	if !ok {
		w.mu.Lock()
		w.callbacks[eventOut.Control] = make(map[input.EventType]input.ControlFunction)
		w.mu.Unlock()
	}
	w.mu.RLock()
	defer w.mu.RUnlock()

	ctrlFunc, ok := w.callbacks[eventOut.Control][eventOut.Event]
	if ok && ctrlFunc != nil {
		ctrlFunc(ctx, eventOut)
	}

	ctrlFuncAll, ok := w.callbacks[eventOut.Control][input.AllEvents]
	if ok && ctrlFuncAll != nil {
		ctrlFuncAll(ctx, eventOut)
	}
}

// Controls lists the inputs of the gamepad.
func (w *webGamepad) Controls(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
	out := append([]input.Control(nil), w.controls...)
	return out, nil
}

// Events returns the last input.Event (the current state).
func (w *webGamepad) Events(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make(map[input.Control]input.Event)
	for key, value := range w.lastEvents {
		out[key] = value
	}
	return out, nil
}

// RegisterControlCallback registers a callback function to be executed on the specified control's trigger Events.
func (w *webGamepad) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
	extra map[string]interface{},
) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.callbacks[control] == nil {
		w.callbacks[control] = make(map[input.EventType]input.ControlFunction)
	}

	for _, trigger := range triggers {
		if trigger == input.ButtonChange {
			w.callbacks[control][input.ButtonRelease] = ctrlFunc
			w.callbacks[control][input.ButtonPress] = ctrlFunc
		} else {
			w.callbacks[control][trigger] = ctrlFunc
		}
	}
	return nil
}

// TriggerEvent allows directly sending an Event (such as a button press) from external code.
func (w *webGamepad) TriggerEvent(ctx context.Context, event input.Event, extra map[string]interface{}) error {
	w.makeCallbacks(ctx, event)
	return nil
}

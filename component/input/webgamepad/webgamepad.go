// Package webgamepad implements a web based input controller.
package webgamepad

import (
	"context"
	"errors"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const (
	modelname = "webgamepad"
	// NOTE: Component NAME (in config file) must also be set to "WebGamepad" exactly
	// This is because there's no way to get a component's model from a robot.Robot.
)

func init() {
	registry.RegisterComponent(input.Subtype, modelname, registry.Component{Constructor: NewController})
}

// NewController creates a new gamepad.
func NewController(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
	var w webGamepad
	w.callbacks = make(map[input.Control]map[input.EventType]input.ControlFunction)
	w.lastEvents = make(map[input.Control]input.Event)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	w.ctxWithCancel = ctxWithCancel
	w.controls = []input.Control{
		input.AbsoluteX, input.AbsoluteY, input.AbsoluteRX, input.AbsoluteRY,
		input.AbsoluteZ, input.AbsoluteRZ, input.AbsoluteHat0X, input.AbsoluteHat0Y,
		input.ButtonSouth, input.ButtonEast, input.ButtonWest, input.ButtonNorth,
		input.ButtonLT, input.ButtonRT, input.ButtonLThumb, input.ButtonRThumb,
		input.ButtonSelect, input.ButtonStart, input.ButtonMenu,
	}
	return &w, nil
}

// webGamepad is an input.Controller.
type webGamepad struct {
	controls                []input.Control
	lastEvents              map[input.Control]input.Event
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	ctxWithCancel           context.Context
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
}

func (w *webGamepad) makeCallbacks(eventOut input.Event) {
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
		w.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer w.activeBackgroundWorkers.Done()
			ctrlFunc(w.ctxWithCancel, eventOut)
		})
	}

	ctrlFuncAll, ok := w.callbacks[eventOut.Control][input.AllEvents]
	if ok && ctrlFuncAll != nil {
		w.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer w.activeBackgroundWorkers.Done()
			ctrlFuncAll(w.ctxWithCancel, eventOut)
		})
	}
}

// Close terminates background worker threads.
func (w *webGamepad) Close() {
	w.cancelFunc()
	w.activeBackgroundWorkers.Wait()
}

// Do is unimplemented.
func (w *webGamepad) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("Do() unimplemented")
}

// GetControls lists the inputs of the gamepad.
func (w *webGamepad) GetControls(ctx context.Context) ([]input.Control, error) {
	out := append([]input.Control(nil), w.controls...)
	return out, nil
}

// GetEvents returns the last input.Event (the current state).
func (w *webGamepad) GetEvents(ctx context.Context) (map[input.Control]input.Event, error) {
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
func (w *webGamepad) TriggerEvent(ctx context.Context, event input.Event) error {
	w.makeCallbacks(event)
	return nil
}

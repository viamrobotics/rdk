package webgamepad

import (
	"context"
	"sync"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

const (
	modelname = "webgamepad"
	// NOTE: Component NAME (in config file) must also be set to "WebGamepad" exactly
	// This is because there's no way to get a component's model from a robot.Robot
)

func init() {
	registry.RegisterInputController(modelname, registry.InputController{Constructor: NewController})
}

// Controller is an input.Controller
type Controller struct {
	controls                []input.Control
	lastEvents              map[input.Control]input.Event
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	ctxWithCancel           context.Context
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
}

func (g *Controller) makeCallbacks(ctx context.Context, eventOut input.Event) {
	g.mu.Lock()
	g.lastEvents[eventOut.Control] = eventOut
	g.mu.Unlock()

	g.mu.RLock()
	_, ok := g.callbacks[eventOut.Control]
	g.mu.RUnlock()
	if !ok {
		g.mu.Lock()
		g.callbacks[eventOut.Control] = make(map[input.EventType]input.ControlFunction)
		g.mu.Unlock()
	}
	g.mu.RLock()
	defer g.mu.RUnlock()

	ctrlFunc, ok := g.callbacks[eventOut.Control][eventOut.Event]
	if ok && ctrlFunc != nil {
		g.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer g.activeBackgroundWorkers.Done()
			ctrlFunc(g.ctxWithCancel, eventOut)
		})
	}

	ctrlFuncAll, ok := g.callbacks[eventOut.Control][input.AllEvents]
	if ok && ctrlFuncAll != nil {
		g.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer g.activeBackgroundWorkers.Done()
			ctrlFuncAll(g.ctxWithCancel, eventOut)
		})
	}
}

// NewController creates a new gamepad
func NewController(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error) {
	var g Controller
	g.callbacks = make(map[input.Control]map[input.EventType]input.ControlFunction)
	g.lastEvents = make(map[input.Control]input.Event)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	g.cancelFunc = cancel
	g.ctxWithCancel = ctxWithCancel
	g.controls = []input.Control{
		input.AbsoluteX, input.AbsoluteY, input.AbsoluteRX, input.AbsoluteRY,
		input.AbsoluteZ, input.AbsoluteRZ, input.AbsoluteHat0X, input.AbsoluteHat0Y,
		input.ButtonSouth, input.ButtonEast, input.ButtonWest, input.ButtonNorth,
		input.ButtonLT, input.ButtonRT, input.ButtonLThumb, input.ButtonRThumb,
		input.ButtonSelect, input.ButtonStart, input.ButtonMenu,
	}
	return &g, nil
}

// Close terminates background worker threads
func (g *Controller) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	return nil
}

// Controls lists the inputs of the gamepad
func (g *Controller) Controls(ctx context.Context) ([]input.Control, error) {
	out := append([]input.Control(nil), g.controls...)
	return out, nil
}

// LastEvents returns the last input.Event (the current state)
func (g *Controller) LastEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[input.Control]input.Event)
	for key, value := range g.lastEvents {
		out[key] = value
	}
	return out, nil
}

// RegisterControlCallback registers a callback function to be executed on the specified control's trigger Events
func (g *Controller) RegisterControlCallback(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.callbacks[control] == nil {
		g.callbacks[control] = make(map[input.EventType]input.ControlFunction)
	}

	for _, trigger := range triggers {
		if trigger == input.ButtonChange {
			g.callbacks[control][input.ButtonRelease] = ctrlFunc
			g.callbacks[control][input.ButtonPress] = ctrlFunc
		} else {
			g.callbacks[control][trigger] = ctrlFunc
		}
	}
	return nil
}

// InjectEvent allows directly sending an Event (such as a button press) from external code
func (g *Controller) InjectEvent(ctx context.Context, event input.Event) error {
	g.makeCallbacks(ctx, event)
	return nil
}

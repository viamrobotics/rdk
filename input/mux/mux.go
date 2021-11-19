package mux

import (
	"context"
	"sync"

	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/multierr"
)

const (
	modelname = "mux"
)

func init() {
	registry.RegisterInputController(modelname, registry.InputController{Constructor: NewController})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeInputController, modelname, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	}, &Config{})

}

// Config is used for converting config attributes
type Config struct {
	Sources []string `json:"sources"`
}

// Controller is an input.Controller
type Controller struct {
	sources                 []input.Controller
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	ctxWithCancel           context.Context
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
	eventsChan              chan input.Event
}

func (g *Controller) makeCallbacks(ctx context.Context, eventOut input.Event) {
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

// NewController returns a new multiplexed input.Controller
func NewController(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error) {
	var g Controller
	g.callbacks = make(map[input.Control]map[input.EventType]input.ControlFunction)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	g.cancelFunc = cancel
	g.ctxWithCancel = ctxWithCancel

	for _, s := range config.ConvertedAttributes.(*Config).Sources {
		c, ok := r.InputControllerByName(s)
		if !ok {
			return nil, errors.Errorf("cannot find input.Controller named: %s", s)
		}
		g.sources = append(g.sources, c)
	}

	g.eventsChan = make(chan input.Event, 1024)

	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()
		for {
			select {
			case eventIn := <-g.eventsChan:
				g.makeCallbacks(ctxWithCancel, eventIn)
			case <-ctxWithCancel.Done():
				return
			}
		}
	})

	return &g, nil
}

// Close terminates background worker threads
func (g *Controller) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	return nil
}

// Controls lists the unique input.Controls for the combined sources
func (g *Controller) Controls(ctx context.Context) ([]input.Control, error) {
	controlMap := make(map[input.Control]bool)
	var ok bool
	var errs error
	for _, c := range g.sources {
		controls, err := c.Controls(ctx)
		if err != nil {
			errs = multierr.Combine(errs, err)
			continue
		}
		ok = true
		for _, ctrl := range controls {
			controlMap[ctrl] = true
		}
	}
	if !ok {
		return nil, errs
	}
	var controlsOut []input.Control
	for c := range controlMap {
		controlsOut = append(controlsOut, c)
	}

	return controlsOut, nil
}

// LastEvents returns the last input.Event (the current state)
func (g *Controller) LastEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	eventsOut := make(map[input.Control]input.Event)
	var ok bool
	var errs error
	for _, c := range g.sources {
		eventList, err := c.LastEvents(ctx)
		if err != nil {
			errs = multierr.Combine(errs, err)
			continue
		}
		ok = true
		for ctrl, eventA := range eventList {
			eventB, ok := eventsOut[ctrl]
			if !ok || eventA.Time.After(eventB.Time) {
				eventsOut[ctrl] = eventA
			}
		}
	}
	if !ok {
		return nil, errs
	}
	return eventsOut, nil
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

	relayFunc := func(ctx context.Context, eventIn input.Event) {
		select {
		case g.eventsChan <- eventIn:
		case <-ctx.Done():
		}
	}

	var ok bool
	var errs error
	for _, c := range g.sources {
		err := c.RegisterControlCallback(ctx, control, triggers, relayFunc)
		if err != nil {
			errs = multierr.Combine(errs, err)
			continue
		}
		ok = true
	}
	if !ok {
		return errs
	}
	return nil
}

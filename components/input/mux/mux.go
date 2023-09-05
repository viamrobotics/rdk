// Package mux implements a multiplexed input controller.
package mux

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("mux")

func init() {
	resource.RegisterComponent(input.API, model, resource.Registration[input.Controller, *Config]{
		Constructor: NewController,
	})
}

// Config is used for converting config attributes.
type Config struct {
	resource.TriviallyValidateConfig
	Sources []string `json:"sources"`
}

// NewController returns a new multiplexed input.Controller.
func NewController(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (input.Controller, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	m := mux{
		Named:         conf.ResourceName().AsNamed(),
		callbacks:     map[input.Control]map[input.EventType]input.ControlFunction{},
		cancelFunc:    cancel,
		ctxWithCancel: ctxWithCancel,
		eventsChan:    make(chan input.Event, 1024),
		logger:        logger,
	}

	for _, s := range newConf.Sources {
		c, err := input.FromDependencies(deps, s)
		if err != nil {
			return nil, err
		}
		m.sources = append(m.sources, c)
	}

	m.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer m.activeBackgroundWorkers.Done()
		for {
			select {
			case eventIn := <-m.eventsChan:
				m.makeCallbacks(eventIn)
			case <-ctxWithCancel.Done():
				return
			}
		}
	})

	return &m, nil
}

// mux is an input.Controller.
type mux struct {
	resource.Named
	resource.AlwaysRebuild

	sources                 []input.Controller
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	ctxWithCancel           context.Context
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
	eventsChan              chan input.Event
	logger                  golog.Logger
}

func (m *mux) makeCallbacks(eventOut input.Event) {
	m.mu.RLock()
	_, ok := m.callbacks[eventOut.Control]
	m.mu.RUnlock()
	if !ok {
		m.mu.Lock()
		m.callbacks[eventOut.Control] = make(map[input.EventType]input.ControlFunction)
		m.mu.Unlock()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctrlFunc, ok := m.callbacks[eventOut.Control][eventOut.Event]
	if ok && ctrlFunc != nil {
		m.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer m.activeBackgroundWorkers.Done()
			ctrlFunc(m.ctxWithCancel, eventOut)
		})
	}

	ctrlFuncAll, ok := m.callbacks[eventOut.Control][input.AllEvents]
	if ok && ctrlFuncAll != nil {
		m.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() {
			defer m.activeBackgroundWorkers.Done()
			ctrlFuncAll(m.ctxWithCancel, eventOut)
		})
	}
}

// Close terminates background worker threads.
func (m *mux) Close(ctx context.Context) error {
	m.cancelFunc()
	m.activeBackgroundWorkers.Wait()
	return nil
}

// Controls lists the unique input.Controls for the combined sources.
func (m *mux) Controls(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
	controlMap := make(map[input.Control]bool)
	var ok bool
	var errs error
	for _, c := range m.sources {
		controls, err := c.Controls(ctx, extra)
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

// Events returns the last input.Event (the current state).
func (m *mux) Events(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
	eventsOut := make(map[input.Control]input.Event)
	var ok bool
	var errs error
	for _, c := range m.sources {
		eventList, err := c.Events(ctx, extra)
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

// RegisterControlCallback registers a callback function to be executed on the specified control's trigger Events.
func (m *mux) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
	extra map[string]interface{},
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callbacks[control] == nil {
		m.callbacks[control] = make(map[input.EventType]input.ControlFunction)
	}

	for _, trigger := range triggers {
		if trigger == input.ButtonChange {
			m.callbacks[control][input.ButtonRelease] = ctrlFunc
			m.callbacks[control][input.ButtonPress] = ctrlFunc
		} else {
			m.callbacks[control][trigger] = ctrlFunc
		}
	}

	relayFunc := func(ctx context.Context, eventIn input.Event) {
		select {
		case m.eventsChan <- eventIn:
		case <-ctx.Done():
		}
	}

	var ok bool
	var errs error
	for _, c := range m.sources {
		err := c.RegisterControlCallback(ctx, control, triggers, relayFunc, extra)
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

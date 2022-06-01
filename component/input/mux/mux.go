// Package mux implements a multiplexed input controller.
package mux

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const (
	modelname = "mux"
)

func init() {
	registry.RegisterComponent(input.Subtype, modelname, registry.Component{Constructor: NewController})

	config.RegisterComponentAttributeMapConverter(
		input.SubtypeName,
		modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{})
}

// Config is used for converting config attributes.
type Config struct {
	Sources []string `json:"sources"`
}

// NewController returns a new multiplexed input.Controller.
func NewController(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
	var m mux
	m.callbacks = make(map[input.Control]map[input.EventType]input.ControlFunction)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel
	m.ctxWithCancel = ctxWithCancel

	for _, s := range config.ConvertedAttributes.(*Config).Sources {
		c, err := input.FromRobot(r, s)
		if err != nil {
			return nil, err
		}
		m.sources = append(m.sources, c)
	}

	m.eventsChan = make(chan input.Event, 1024)

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
	sources                 []input.Controller
	mu                      sync.RWMutex
	activeBackgroundWorkers sync.WaitGroup
	ctxWithCancel           context.Context
	cancelFunc              func()
	callbacks               map[input.Control]map[input.EventType]input.ControlFunction
	eventsChan              chan input.Event
	generic.Unimplemented
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
func (m *mux) Close() {
	m.cancelFunc()
	m.activeBackgroundWorkers.Wait()
}

// GetControls lists the unique input.GetControls for the combined sources.
func (m *mux) GetControls(ctx context.Context) ([]input.Control, error) {
	controlMap := make(map[input.Control]bool)
	var ok bool
	var errs error
	for _, c := range m.sources {
		controls, err := c.GetControls(ctx)
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

// GetEvents returns the last input.Event (the current state).
func (m *mux) GetEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	eventsOut := make(map[input.Control]input.Event)
	var ok bool
	var errs error
	for _, c := range m.sources {
		eventList, err := c.GetEvents(ctx)
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

// Package fake implements a fake input controller.
package fake

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var modelName = resource.NewDefaultModel("fake")

func init() {
	registry.RegisterComponent(
		input.Subtype,
		modelName,
		registry.Component{
			Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, _ golog.Logger) (interface{}, error) {
				return NewInputController(ctx, config)
			},
		},
	)

	config.RegisterComponentAttributeMapConverter(
		input.Subtype,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Squash: true, Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		},
		&Config{},
	)
}

// Config can list input structs (with their states), define event values and callback delays.
type Config struct {
	controls []input.Control

	// EventValue will dictate the value of the events returned. Random between -1 to 1 if unset.
	EventValue *float64 `json:"event_value,omitempty"`

	// CallbackDelaySec is the amount of time between callbacks getting triggered. Random between (1-2] sec if unset.
	// 0 is not valid and will be overwritten by a random delay.
	CallbackDelaySec float64 `json:"callback_delay_sec"`
}

var _ = input.Controller(&InputController{})

type callback struct {
	control  input.Control
	triggers []input.EventType
	ctrlFunc input.ControlFunction
}

// NewInputController returns a fake input.Controller.
func NewInputController(ctx context.Context, config config.Component) (input.Controller, error) {
	cfg := config.ConvertedAttributes.(*Config)

	closeCtx, cancelFunc := context.WithCancel(ctx)

	c := &InputController{
		closeCtx:   closeCtx,
		cancelFunc: cancelFunc,
		controls:   cfg.controls,
		eventValue: cfg.EventValue,
		callbacks:  make([]callback, 0),
	}

	if cfg.CallbackDelaySec != 0 {
		// convert to milliseconds to avoid any issues with float to int conversions
		delay := time.Duration(cfg.CallbackDelaySec*1000) * time.Millisecond
		c.callbackDelay = &delay
	}

	// start callback thread
	c.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		c.startCallbackLoop()
	}, c.activeBackgroundWorkers.Done)

	return c, nil
}

// An InputController fakes an input.Controller.
type InputController struct {
	mu sync.Mutex

	closeCtx                context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	Name          string
	controls      []input.Control
	eventValue    *float64
	callbackDelay *time.Duration
	callbacks     []callback
	generic.Echo
}

// Controls lists the inputs of the gamepad.
func (c *InputController) Controls(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.controls) == 0 {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	return c.controls, nil
}

func (c *InputController) eventVal() float64 {
	if c.eventValue != nil {
		return *c.eventValue
	}
	//nolint:gosec
	return rand.Float64()
}

// Events returns the a specified or random input.Event (the current state) for AbsoluteX.
func (c *InputController) Events(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	eventsOut := make(map[input.Control]input.Event)

	eventsOut[input.AbsoluteX] = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: c.eventVal()}
	return eventsOut, nil
}

// RegisterControlCallback registers a callback function to be executed on the specified trigger Event. The fake implementation will
// trigger the callback at a random or user-specified interval with a random or user-specified value.
func (c *InputController) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
	extra map[string]interface{},
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.callbacks = append(c.callbacks, callback{control: control, triggers: triggers, ctrlFunc: ctrlFunc})
	return nil
}

func (c *InputController) startCallbackLoop() {
	var callbackDelay time.Duration
	var evValue float64

	for {
		if c.callbackDelay != nil {
			callbackDelay = *c.callbackDelay
		} else {
			//nolint:gosec
			callbackDelay = 1*time.Second + time.Duration(rand.Float64()*1000)*time.Millisecond
		}
		if !utils.SelectContextOrWait(c.closeCtx, callbackDelay) {
			return
		}

		select {
		case <-c.closeCtx.Done():
			return
		default:
			c.mu.Lock()
			evValue = c.eventVal()
			for _, callback := range c.callbacks {
				for _, t := range callback.triggers {
					event := input.Event{Time: time.Now(), Event: t, Control: callback.control, Value: evValue}
					callback.ctrlFunc(c.closeCtx, event)
				}
			}
			c.mu.Unlock()
		}
	}
}

// TriggerEvent allows directly sending an Event (such as a button press) from external code.
func (c *InputController) TriggerEvent(ctx context.Context, event input.Event, extra map[string]interface{}) error {
	return errors.New("unsupported")
}

// Close attempts to cleanly close the input controller.
func (c *InputController) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var err error
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}

	c.activeBackgroundWorkers.Wait()
	return err
}

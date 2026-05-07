// Package fake implements a fake input controller.
package fake

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("fake")

func init() {
	resource.RegisterComponent(
		input.API,
		model,
		resource.Registration[input.Controller, *Config]{
			Constructor: func(
				ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
			) (input.Controller, error) {
				return NewInputController(ctx, conf, logger)
			},
		},
	)
}

// Config can list input structs (with their states), define event values and callback delays.
type Config struct {
	resource.TriviallyValidateConfig
	controls []input.Control

	// EventValue will dictate the value of the events returned. Random between -1 to 1 if unset.
	EventValue *float64 `json:"event_value,omitempty"`

	// CallbackDelaySec is the amount of time between callbacks getting triggered. Random between (1-2] sec if unset.
	// 0 is not valid and will be overwritten by a random delay.
	CallbackDelaySec float64 `json:"callback_delay_sec"`
}

type callback struct {
	control  input.Control
	triggers []input.EventType
	ctrlFunc input.ControlFunction
}

// NewInputController returns a fake input.Controller.
func NewInputController(ctx context.Context, conf resource.Config, logger logging.Logger) (input.Controller, error) {
	closeCtx, cancelFunc := context.WithCancel(context.Background())

	c := &InputController{
		Named:      conf.ResourceName().AsNamed(),
		closeCtx:   closeCtx,
		cancelFunc: cancelFunc,
		callbacks:  make([]callback, 0),
		logger:     logger,
	}

	if err := c.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
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
	resource.Named

	closeCtx                context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	mu            sync.Mutex
	controls      []input.Control
	eventValue    *float64
	callbackDelay *time.Duration
	callbacks     []callback
	logger        logging.Logger
}

// Reconfigure updates the config of the controller.
func (c *InputController) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.controls = newConf.controls
	c.eventValue = newConf.EventValue
	if newConf.CallbackDelaySec != 0 {
		// convert to milliseconds to avoid any issues with float to int conversions
		delay := time.Duration(newConf.CallbackDelaySec*1000) * time.Millisecond
		c.callbackDelay = &delay
	}
	return nil
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
	for {
		var callbackDelay time.Duration

		if c.closeCtx.Err() != nil {
			return
		}

		c.mu.Lock()
		if c.callbackDelay != nil {
			callbackDelay = *c.callbackDelay
		} else {
			//nolint:gosec
			callbackDelay = 1*time.Second + time.Duration(rand.Float64()*1000)*time.Millisecond
		}
		c.mu.Unlock()

		if !utils.SelectContextOrWait(c.closeCtx, callbackDelay) {
			return
		}

		select {
		case <-c.closeCtx.Done():
			return
		default:
			c.mu.Lock()
			evValue := c.eventVal()
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
	var err error
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}
	c.mu.Unlock()

	c.activeBackgroundWorkers.Wait()
	return err
}

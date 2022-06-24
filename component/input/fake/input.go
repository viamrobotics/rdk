// Package fake implements a fake input controller.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const modelName = "fake"

func init() {
	registry.RegisterComponent(input.Subtype, modelName, registry.Component{Constructor: NewInputController})

	config.RegisterComponentAttributeMapConverter(
		input.SubtypeName,
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

// NewInputController returns a fake input.Controller.
func NewInputController(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
	c := &InputController{}
	c.controls = config.ConvertedAttributes.(*Config).controls
	return c, nil
}

// Config can list input structs (with their states).
type Config struct {
	controls []input.Control
}

// An InputController fakes an input.Controller.
type InputController struct {
	Name     string
	mu       sync.Mutex
	controls []input.Control
	generic.Echo
}

// GetControls lists the inputs of the gamepad.
func (c *InputController) GetControls(ctx context.Context) ([]input.Control, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.controls) == 0 {
		return []input.Control{input.AbsoluteX, input.ButtonStart}, nil
	}
	return c.controls, nil
}

// GetEvents returns the last input.Event (the current state) of each control.
func (c *InputController) GetEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	eventsOut := make(map[input.Control]input.Event)
	eventsOut[input.AbsoluteX] = input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
	return eventsOut, nil
}

// RegisterControlCallback registers a callback function to be executed on the specified trigger Event.
func (c *InputController) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
) error {
	return errors.New("unsupported")
}

// TriggerEvent allows directly sending an Event (such as a button press) from external code.
func (c *InputController) TriggerEvent(ctx context.Context, event input.Event) error {
	return errors.New("unsupported")
}

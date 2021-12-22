package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/core/component/input"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/mitchellh/mapstructure"
)

func init() {
	registry.RegisterComponent(input.Subtype, "fake", registry.Component{Constructor: NewInputController})

	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeInputController,
		"fake",
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
		}, &Config{})
}

// NewInputController returns a fake input.Controller
func NewInputController(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
	c := &InputController{}
	c.controls = config.ConvertedAttributes.(*Config).controls
	return c, nil
}

// Config can list input structs (with their states)
type Config struct {
	controls []input.Control
}

// An InputController fakes an input.Controller
type InputController struct {
	Name       string
	mu         sync.Mutex
	controls   []input.Control
	lastEvents map[input.Control]input.Event
}

// Controls lists the inputs of the gamepad
func (c *InputController) Controls(ctx context.Context) ([]input.Control, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.controls, nil
}

// LastEvents returns the last input.Event (the current state) of each control
func (c *InputController) LastEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastEvents, nil
}

// RegisterControlCallback registers a callback function to be executed on the specified trigger Event
func (c *InputController) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
) error {
	return errors.New("unsupported")
}

// InjectEvent allows directly sending an Event (such as a button press) from external code
func (c *InputController) InjectEvent(ctx context.Context, event input.Event) error {
	return errors.New("unsupported")
}

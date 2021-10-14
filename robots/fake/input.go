package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/mitchellh/mapstructure"
)

func init() {

	registry.RegisterInputController(modelName, registry.InputController{Constructor: NewInputController})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeInputController, modelName, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Squash: true, Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	})
}

// NewInputController returns a fake input.Controller
func NewInputController(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error) {
	c := &InputController{}

	c.inputs = make(map[input.ControlCode]*Input)

	for _, v := range config.ConvertedAttributes.(*Config).inputs {
		c.inputs[v.controlCode] = v
	}

	return c, nil
}

// Config can list input structs (with their states)
type Config struct {
	inputs []*Input
}

// An InputController fakes an input.Controller
type InputController struct {
	Name   string
	mu     sync.Mutex
	inputs map[input.ControlCode]*Input
}

// Inputs lists the inputs of the gamepad
func (c *InputController) Inputs(ctx context.Context) (map[input.ControlCode]input.Input, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ret := make(map[input.ControlCode]input.Input)
	for k, v := range c.inputs {
		ret[k] = v
	}
	return ret, nil
}

// Input is a fake input.Input
type Input struct {
	controlCode input.ControlCode
	mu          sync.Mutex
	lastEvent   input.Event
}

// Name returns the stringified ControlCode of the input
func (i *Input) Name(ctx context.Context) input.ControlCode {
	return i.controlCode
}

// LastEvent returns the last input.Event (the current state)
func (i *Input) LastEvent(ctx context.Context) (input.Event, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.lastEvent, nil
}

// RegisterControl registers a callback function to be executed on the specified trigger Event
func (i *Input) RegisterControl(ctx context.Context, ctrlFunc input.ControlFunction, trigger input.EventType) error {
	return errors.New("unsupported")
}

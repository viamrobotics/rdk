package inject

import (
	"context"

	"go.viam.com/core/input"
)

// InputController is an injected InputController.
type InputController struct {
	input.Controller
	InputsFunc func(ctx context.Context) (map[input.ControlCode]input.Input, error)
}

// Inputs calls the injected function or the real version.
func (s *InputController) Inputs(ctx context.Context) (map[input.ControlCode]input.Input, error) {
	if s.InputsFunc == nil {
		return s.Controller.Inputs(ctx)
	}
	return s.InputsFunc(ctx)
}

// Input is an injectable input.Input
type Input struct {
	input.Input
	NameFunc            func(ctx context.Context) string
	StateFunc           func(ctx context.Context) (input.Event, error)
	RegisterControlFunc func(ctx context.Context, ctrlFunc input.ControlFunction, trigger input.EventType) error
}

// Name calls the injected function or the real version.
func (i *Input) Name(ctx context.Context) string {
	if i.NameFunc == nil {
		return i.Input.Name(ctx)
	}
	return i.NameFunc(ctx)
}

// State calls the injected function or the real version.
func (i *Input) State(ctx context.Context) (input.Event, error) {
	if i.StateFunc == nil {
		return i.Input.State(ctx)
	}
	return i.StateFunc(ctx)
}

//RegisterControl calls the injected function or the real version.
func (i *Input) RegisterControl(ctx context.Context, ctrlFunc input.ControlFunction, trigger input.EventType) error {
	if i.RegisterControlFunc == nil {
		return i.RegisterControl(ctx, ctrlFunc, trigger)
	}
	return i.RegisterControlFunc(ctx, ctrlFunc, trigger)
}

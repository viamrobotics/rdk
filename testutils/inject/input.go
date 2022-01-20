package inject

import (
	"context"

	"go.viam.com/rdk/component/input"
)

// InputController is an injected InputController.
type InputController struct {
	input.Controller
	ControlsFunc                func(ctx context.Context) ([]input.Control, error)
	GetEventsFunc               func(ctx context.Context) (map[input.Control]input.Event, error)
	RegisterControlCallbackFunc func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error
}

// Controls calls the injected function or the real version.
func (s *InputController) Controls(ctx context.Context) ([]input.Control, error) {
	if s.ControlsFunc == nil {
		return s.Controller.Controls(ctx)
	}
	return s.ControlsFunc(ctx)
}

// GetEvents calls the injected function or the real version.
func (s *InputController) GetEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	if s.GetEventsFunc == nil {
		return s.Controller.GetEvents(ctx)
	}
	return s.GetEventsFunc(ctx)
}

// RegisterControlCallback calls the injected function or the real version.
func (s *InputController) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
) error {
	if s.RegisterControlCallbackFunc == nil {
		return s.RegisterControlCallback(ctx, control, triggers, ctrlFunc)
	}
	return s.RegisterControlCallbackFunc(ctx, control, triggers, ctrlFunc)
}

// InjectableInputController is an injected injectable InputController.
type InjectableInputController struct {
	InputController
	input.Injectable

	InjectEventFunc func(ctx context.Context, event input.Event) error
}

// InjectEvent calls the injected function or the real version.
func (s *InjectableInputController) InjectEvent(ctx context.Context, event input.Event) error {
	if s.InjectEventFunc == nil {
		return s.InjectEvent(ctx, event)
	}
	return s.InjectEventFunc(ctx, event)
}

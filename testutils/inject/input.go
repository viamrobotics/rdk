package inject

import (
	"context"

	"go.viam.com/core/component/input"
)

// InputController is an injected InputController.
type InputController struct {
	input.Controller
	ControlsFunc                func(ctx context.Context) ([]input.Control, error)
	LastEventsFunc              func(ctx context.Context) (map[input.Control]input.Event, error)
	RegisterControlCallbackFunc func(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error
}

// Controls calls the injected function or the real version.
func (s *InputController) Controls(ctx context.Context) ([]input.Control, error) {
	if s.ControlsFunc == nil {
		return s.Controller.Controls(ctx)
	}
	return s.ControlsFunc(ctx)
}

// LastEvents calls the injected function or the real version.
func (s *InputController) LastEvents(ctx context.Context) (map[input.Control]input.Event, error) {
	if s.LastEventsFunc == nil {
		return s.Controller.LastEvents(ctx)
	}
	return s.LastEventsFunc(ctx)
}

//RegisterControlCallback calls the injected function or the real version.
func (s *InputController) RegisterControlCallback(ctx context.Context, control input.Control, triggers []input.EventType, ctrlFunc input.ControlFunction) error {
	if s.RegisterControlCallbackFunc == nil {
		return s.RegisterControlCallback(ctx, control, triggers, ctrlFunc)
	}
	return s.RegisterControlCallbackFunc(ctx, control, triggers, ctrlFunc)
}

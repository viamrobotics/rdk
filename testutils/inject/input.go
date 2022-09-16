package inject

import (
	"context"

	"go.viam.com/rdk/components/input"
)

// InputController is an injected InputController.
type InputController struct {
	input.Controller
	DoFunc                      func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ControlsFunc                func(ctx context.Context) ([]input.Control, error)
	EventsFunc                  func(ctx context.Context) (map[input.Control]input.Event, error)
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

// Events calls the injected function or the real version.
func (s *InputController) Events(ctx context.Context) (map[input.Control]input.Event, error) {
	if s.EventsFunc == nil {
		return s.Controller.Events(ctx)
	}
	return s.EventsFunc(ctx)
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

// DoCommand calls the injected DoCommand or the real version.
func (s *InputController) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Controller.DoCommand(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}

// TriggerableInputController is an injected injectable InputController.
type TriggerableInputController struct {
	InputController
	input.Triggerable

	TriggerEventFunc func(ctx context.Context, event input.Event) error
}

// TriggerEvent calls the injected function or the real version.
func (s *TriggerableInputController) TriggerEvent(ctx context.Context, event input.Event) error {
	if s.TriggerEventFunc == nil {
		return s.TriggerEvent(ctx, event)
	}
	return s.TriggerEventFunc(ctx, event)
}

package inject

import (
	"context"

	"go.viam.com/rdk/component/input"
)

// InputController is an injected InputController.
type InputController struct {
	input.Controller
	DoFunc                      func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetControlsFunc             func(ctx context.Context) ([]input.Control, error)
	GetEventsFunc               func(ctx context.Context) (map[input.Control]input.Event, error)
	RegisterControlCallbackFunc func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error
}

// GetControls calls the injected function or the real version.
func (s *InputController) GetControls(ctx context.Context) ([]input.Control, error) {
	if s.GetControlsFunc == nil {
		return s.Controller.GetControls(ctx)
	}
	return s.GetControlsFunc(ctx)
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

// Do calls the injected Do or the real version.
func (s *InputController) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Controller.Do(ctx, cmd)
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

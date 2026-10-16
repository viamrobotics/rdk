package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/statemachine"
)

// StateMachine is an injected state machine service for testing.
type StateMachine struct {
	statemachine.Service
	name              resource.Name
	GetStateFunc      func(ctx context.Context, extra map[string]interface{}) (statemachine.State, error)
	SendEventFunc     func(ctx context.Context, event string, payload, extra map[string]interface{}) (statemachine.EventResult, error)
	GetHistoryFunc    func(ctx context.Context, limit int, extra map[string]interface{}) ([]statemachine.Transition, error)
	GetDefinitionFunc func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error)
	StopFunc          func(ctx context.Context, extra map[string]interface{}) error
	ResetFunc         func(ctx context.Context, extra map[string]interface{}) error
	DoFunc            func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc         func(ctx context.Context) error
}

// NewStateMachine returns a new injected state machine service.
func NewStateMachine(name string) *StateMachine {
	return &StateMachine{name: statemachine.Named(name)}
}

// Name returns the name of the injected state machine.
func (s *StateMachine) Name() resource.Name {
	return s.name
}

// GetState calls the injected GetStateFunc or the real version.
func (s *StateMachine) GetState(ctx context.Context, extra map[string]interface{}) (statemachine.State, error) {
	if s.GetStateFunc == nil {
		return s.Service.GetState(ctx, extra)
	}
	return s.GetStateFunc(ctx, extra)
}

// SendEvent calls the injected SendEventFunc or the real version.
func (s *StateMachine) SendEvent(
	ctx context.Context, event string, payload, extra map[string]interface{},
) (statemachine.EventResult, error) {
	if s.SendEventFunc == nil {
		return s.Service.SendEvent(ctx, event, payload, extra)
	}
	return s.SendEventFunc(ctx, event, payload, extra)
}

// GetHistory calls the injected GetHistoryFunc or the real version.
func (s *StateMachine) GetHistory(ctx context.Context, limit int, extra map[string]interface{}) ([]statemachine.Transition, error) {
	if s.GetHistoryFunc == nil {
		return s.Service.GetHistory(ctx, limit, extra)
	}
	return s.GetHistoryFunc(ctx, limit, extra)
}

// GetDefinition calls the injected GetDefinitionFunc or the real version.
func (s *StateMachine) GetDefinition(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	if s.GetDefinitionFunc == nil {
		return s.Service.GetDefinition(ctx, extra)
	}
	return s.GetDefinitionFunc(ctx, extra)
}

// Stop calls the injected StopFunc or the real version.
func (s *StateMachine) Stop(ctx context.Context, extra map[string]interface{}) error {
	if s.StopFunc == nil {
		return s.Service.Stop(ctx, extra)
	}
	return s.StopFunc(ctx, extra)
}

// Reset calls the injected ResetFunc or the real version.
func (s *StateMachine) Reset(ctx context.Context, extra map[string]interface{}) error {
	if s.ResetFunc == nil {
		return s.Service.Reset(ctx, extra)
	}
	return s.ResetFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (s *StateMachine) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Service.DoCommand(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}

// Status returns an empty status unless a Service is set.
func (s *StateMachine) Status(ctx context.Context) (map[string]interface{}, error) {
	if s.Service != nil {
		return s.Service.Status(ctx)
	}
	return map[string]interface{}{}, nil
}

// Close calls the injected Close or the real version.
func (s *StateMachine) Close(ctx context.Context) error {
	if s.CloseFunc == nil {
		if s.Service == nil {
			return nil
		}
		return s.Service.Close(ctx)
	}
	return s.CloseFunc(ctx)
}

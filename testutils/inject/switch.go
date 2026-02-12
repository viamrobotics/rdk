package inject

import (
	"context"

	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/resource"
)

// Switch is an injected switch.
type Switch struct {
	toggleswitch.Switch
	name                     resource.Name
	SetPositionFunc          func(ctx context.Context, position uint32, extra map[string]interface{}) error
	GetPositionFunc          func(ctx context.Context, extra map[string]interface{}) (uint32, error)
	GetNumberOfPositionsFunc func(ctx context.Context, extra map[string]interface{}) (uint32, []string, error)
	DoFunc                   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc                func(ctx context.Context) error
}

// NewSwitch returns a new injected switch.
func NewSwitch(name string) *Switch {
	return &Switch{name: toggleswitch.Named(name)}
}

// Name returns the name of the resource.
func (s *Switch) Name() resource.Name {
	return s.name
}

// SetPosition sets the switch position.
func (s *Switch) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	if s.SetPositionFunc == nil {
		return s.Switch.SetPosition(ctx, position, extra)
	}
	return s.SetPositionFunc(ctx, position, extra)
}

// GetPosition gets the current switch position.
func (s *Switch) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	if s.GetPositionFunc == nil {
		return s.Switch.GetPosition(ctx, extra)
	}
	return s.GetPositionFunc(ctx, extra)
}

// GetNumberOfPositions gets the total number of positions for the switch.
func (s *Switch) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	if s.GetNumberOfPositionsFunc == nil {
		return s.Switch.GetNumberOfPositions(ctx, extra)
	}
	return s.GetNumberOfPositionsFunc(ctx, extra)
}

// DoCommand calls DoFunc.
func (s *Switch) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Switch.DoCommand(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}

// Close calls CloseFunc.
func (s *Switch) Close(ctx context.Context) error {
	if s.CloseFunc == nil {
		return s.Switch.Close(ctx)
	}
	return s.CloseFunc(ctx)
}

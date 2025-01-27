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
	DoFunc                   func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	SetPositionFunc          func(ctx context.Context, position uint32, extra map[string]interface{}) error
	GetPositionFunc          func(ctx context.Context, extra map[string]interface{}) (uint32, error)
	GetNumberOfPositionsFunc func(ctx context.Context, extra map[string]interface{}) (uint32, error)
}

// NewSwitch returns a new injected switch.
func NewSwitch(name string) *Switch {
	return &Switch{name: toggleswitch.Named(name)}
}

// Name returns the name of the resource.
func (s *Switch) Name() resource.Name {
	return s.name
}

// DoCommand executes a command on the switch.
func (s *Switch) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return nil, nil
	}
	return s.DoFunc(ctx, cmd)
}

// SetPosition sets the switch position.
func (s *Switch) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	if s.SetPositionFunc == nil {
		return nil
	}
	return s.SetPositionFunc(ctx, position, extra)
}

// GetPosition gets the current switch position.
func (s *Switch) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	if s.GetPositionFunc == nil {
		return 0, nil
	}
	return s.GetPositionFunc(ctx, extra)
}

// GetNumberOfPositions gets the total number of positions for the switch.
func (s *Switch) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	if s.GetNumberOfPositionsFunc == nil {
		return 0, nil
	}
	return s.GetNumberOfPositionsFunc(ctx, extra)
}

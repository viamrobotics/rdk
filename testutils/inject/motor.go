package inject

import (
	"context"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
)

// Motor is an injected motor.
type Motor struct {
	motor.Motor
	name                  resource.Name
	DoFunc                func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	SetPowerFunc          func(ctx context.Context, powerPct float64, extra map[string]interface{}) error
	GoForFunc             func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error
	GoToFunc              func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error
	SetRPMFunc            func(ctx context.Context, rpm float64, extra map[string]interface{}) error
	ResetZeroPositionFunc func(ctx context.Context, offset float64, extra map[string]interface{}) error
	PositionFunc          func(ctx context.Context, extra map[string]interface{}) (float64, error)
	PropertiesFunc        func(ctx context.Context, extra map[string]interface{}) (motor.Properties, error)
	StopFunc              func(ctx context.Context, extra map[string]interface{}) error
	IsPoweredFunc         func(ctx context.Context, extra map[string]interface{}) (bool, float64, error)
	IsMovingFunc          func(context.Context) (bool, error)
}

// NewMotor returns a new injected motor.
func NewMotor(name string) *Motor {
	return &Motor{name: motor.Named(name)}
}

// Name returns the name of the resource.
func (m *Motor) Name() resource.Name {
	return m.name
}

// SetPower calls the injected Power or the real version.
func (m *Motor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	if m.SetPowerFunc == nil {
		return m.Motor.SetPower(ctx, powerPct, extra)
	}
	return m.SetPowerFunc(ctx, powerPct, extra)
}

// GoFor calls the injected GoFor or the real version.
func (m *Motor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(ctx, rpm, revolutions, extra)
	}
	return m.GoForFunc(ctx, rpm, revolutions, extra)
}

// GoTo calls the injected GoTo or the real version.
func (m *Motor) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	if m.GoToFunc == nil {
		return m.Motor.GoTo(ctx, rpm, positionRevolutions, extra)
	}
	return m.GoToFunc(ctx, rpm, positionRevolutions, extra)
}

// SetRPM calls the injected SetRPM or the real version.
func (m *Motor) SetRPM(ctx context.Context, rpm float64, extra map[string]interface{}) error {
	if m.SetRPMFunc == nil {
		return m.Motor.SetRPM(ctx, rpm, extra)
	}
	return m.SetRPMFunc(ctx, rpm, extra)
}

// ResetZeroPosition calls the injected Zero or the real version.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	if m.ResetZeroPositionFunc == nil {
		return m.Motor.ResetZeroPosition(ctx, offset, extra)
	}
	return m.ResetZeroPositionFunc(ctx, offset, extra)
}

// Position calls the injected Position or the real version.
func (m *Motor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	if m.PositionFunc == nil {
		return m.Motor.Position(ctx, extra)
	}
	return m.PositionFunc(ctx, extra)
}

// Properties calls the injected Properties or the real version.
func (m *Motor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	if m.PropertiesFunc == nil {
		return m.Motor.Properties(ctx, extra)
	}
	return m.PropertiesFunc(ctx, extra)
}

// Stop calls the injected Off or the real version.
func (m *Motor) Stop(ctx context.Context, extra map[string]interface{}) error {
	if m.StopFunc == nil {
		return m.Motor.Stop(ctx, extra)
	}
	return m.StopFunc(ctx, extra)
}

// IsPowered calls the injected IsPowered or the real version.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	if m.IsPoweredFunc == nil {
		return m.Motor.IsPowered(ctx, extra)
	}
	return m.IsPoweredFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (m *Motor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if m.DoFunc == nil {
		return m.Motor.DoCommand(ctx, cmd)
	}
	return m.DoFunc(ctx, cmd)
}

// IsMoving calls the injected IsMoving or the real version.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	if m.IsMovingFunc == nil {
		return m.Motor.IsMoving(ctx)
	}
	return m.IsMovingFunc(ctx)
}

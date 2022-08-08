package inject

import (
	"context"

	"go.viam.com/rdk/component/motor"
)

// Motor is an injected motor.
type Motor struct {
	motor.Motor
	DoFunc                func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	SetPowerFunc          func(ctx context.Context, powerPct float64, extra map[string]interface{}) error
	GoForFunc             func(ctx context.Context, rpm float64, rotations float64, extra map[string]interface{}) error
	GoToFunc              func(ctx context.Context, rpm float64, position float64, extra map[string]interface{}) error
	ResetZeroPositionFunc func(ctx context.Context, offset float64, extra map[string]interface{}) error
	GetPositionFunc       func(ctx context.Context, extra map[string]interface{}) (float64, error)
	GetFeaturesFunc       func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error)
	StopFunc              func(ctx context.Context, extra map[string]interface{}) error
	IsPoweredFunc         func(ctx context.Context, extra map[string]interface{}) (bool, error)
}

// SetPower calls the injected Power or the real version.
func (m *Motor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	if m.SetPowerFunc == nil {
		return m.Motor.SetPower(ctx, powerPct, extra)
	}
	return m.SetPowerFunc(ctx, powerPct, extra)
}

// GoFor calls the injected GoFor or the real version.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64, extra map[string]interface{}) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(ctx, rpm, revolutions, extra)
	}
	return m.GoForFunc(ctx, rpm, revolutions, extra)
}

// GoTo calls the injected GoTo or the real version.
func (m *Motor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
	if m.GoToFunc == nil {
		return m.Motor.GoTo(ctx, rpm, positionRevolutions, extra)
	}
	return m.GoToFunc(ctx, rpm, positionRevolutions, extra)
}

// ResetZeroPosition calls the injected Zero or the real version.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	if m.ResetZeroPositionFunc == nil {
		return m.Motor.ResetZeroPosition(ctx, offset, extra)
	}
	return m.ResetZeroPositionFunc(ctx, offset, extra)
}

// GetPosition calls the injected Position or the real version.
func (m *Motor) GetPosition(ctx context.Context, extra map[string]interface{}) (float64, error) {
	if m.GetPositionFunc == nil {
		return m.Motor.GetPosition(ctx, extra)
	}
	return m.GetPositionFunc(ctx, extra)
}

// GetFeatures calls the injected GetFeatures or the real version.
func (m *Motor) GetFeatures(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	if m.GetFeaturesFunc == nil {
		return m.Motor.GetFeatures(ctx, extra)
	}
	return m.GetFeaturesFunc(ctx, extra)
}

// Stop calls the injected Off or the real version.
func (m *Motor) Stop(ctx context.Context, extra map[string]interface{}) error {
	if m.StopFunc == nil {
		return m.Motor.Stop(ctx, extra)
	}
	return m.StopFunc(ctx, extra)
}

// IsPowered calls the injected IsPowered or the real version.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, error) {
	if m.IsPoweredFunc == nil {
		return m.Motor.IsPowered(ctx, extra)
	}
	return m.IsPoweredFunc(ctx, extra)
}

// Do calls the injected Do or the real version.
func (m *Motor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if m.DoFunc == nil {
		return m.Motor.Do(ctx, cmd)
	}
	return m.DoFunc(ctx, cmd)
}

// LocalMotor is an injected motor that supports additional features provided by RDK
// (e.g. GoTillStop).
type LocalMotor struct {
	Motor
	GoTillStopFunc func(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error
	IsMovingFunc   func(context.Context) (bool, error)
}

// GoTillStop calls the injected GoTillStop or the real version.
func (m *LocalMotor) GoTillStop(
	ctx context.Context, rpm float64,
	stopFunc func(ctx context.Context) bool,
) error {
	if m.GoTillStopFunc == nil {
		stoppableMotor, ok := m.Motor.Motor.(motor.LocalMotor)
		if !ok {
			return motor.NewGoTillStopUnsupportedError("(name unavailable)")
		}
		return stoppableMotor.GoTillStop(ctx, rpm, stopFunc)
	}
	return m.GoTillStopFunc(ctx, rpm, stopFunc)
}

// IsMoving calls the injected IsMoving or the real version.
func (m *LocalMotor) IsMoving(ctx context.Context) (bool, error) {
	if m.IsMovingFunc == nil {
		return m.Motor.Motor.(motor.LocalMotor).IsMoving(ctx)
	}
	return m.IsMovingFunc(ctx)
}

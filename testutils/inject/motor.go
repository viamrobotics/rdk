package inject

import (
	"context"
	"errors"

	"go.viam.com/rdk/component/motor"
)

// Motor is an injected motor.
type Motor struct {
	motor.Motor
	SetPowerFunc          func(ctx context.Context, powerPct float64) error
	GoForFunc             func(ctx context.Context, rpm float64, rotations float64) error
	GoToFunc              func(ctx context.Context, rpm float64, position float64) error
	GoTillStopFunc        func(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error
	ResetZeroPositionFunc func(ctx context.Context, offset float64) error
	PositionFunc          func(ctx context.Context) (float64, error)
	PositionSupportedFunc func(ctx context.Context) (bool, error)
	StopFunc              func(ctx context.Context) error
	IsInMotionFunc        func(ctx context.Context) (bool, error)
}

// SetPower calls the injected Power or the real version.
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	if m.SetPowerFunc == nil {
		return m.Motor.SetPower(ctx, powerPct)
	}
	return m.SetPowerFunc(ctx, powerPct)
}

// GoFor calls the injected GoFor or the real version.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(ctx, rpm, revolutions)
	}
	return m.GoForFunc(ctx, rpm, revolutions)
}

// GoTo calls the injected GoTo or the real version.
func (m *Motor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	if m.GoToFunc == nil {
		return m.Motor.GoTo(ctx, rpm, positionRevolutions)
	}
	return m.GoToFunc(ctx, rpm, positionRevolutions)
}

// GoTillStop calls the injected GoTillStop or the real version.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if m.GoTillStopFunc == nil {
		stoppableMotor, ok := m.Motor.(motor.GoTillStopSupportingMotor)
		if !ok {
			return errors.New("underlying motor does not implement GoTillStop")
		}
		return stoppableMotor.GoTillStop(ctx, rpm, stopFunc)
	}
	return m.GoTillStopFunc(ctx, rpm, stopFunc)
}

// ResetZeroPosition calls the injected Zero or the real version.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	if m.ResetZeroPositionFunc == nil {
		return m.Motor.ResetZeroPosition(ctx, offset)
	}
	return m.ResetZeroPositionFunc(ctx, offset)
}

// Position calls the injected Position or the real version.
func (m *Motor) Position(ctx context.Context) (float64, error) {
	if m.PositionFunc == nil {
		return m.Motor.Position(ctx)
	}
	return m.PositionFunc(ctx)
}

// PositionSupported calls the injected PositionSupported or the real version.
func (m *Motor) PositionSupported(ctx context.Context) (bool, error) {
	if m.PositionSupportedFunc == nil {
		return m.Motor.PositionSupported(ctx)
	}
	return m.PositionSupportedFunc(ctx)
}

// Stop calls the injected Off or the real version.
func (m *Motor) Stop(ctx context.Context) error {
	if m.StopFunc == nil {
		return m.Motor.Stop(ctx)
	}
	return m.StopFunc(ctx)
}

// IsInMotion calls the injected IsInMotion or the real version.
func (m *Motor) IsInMotion(ctx context.Context) (bool, error) {
	if m.IsInMotionFunc == nil {
		return m.Motor.IsInMotion(ctx)
	}
	return m.IsInMotionFunc(ctx)
}

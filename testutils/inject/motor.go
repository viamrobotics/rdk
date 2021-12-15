package inject

import (
	"context"

	"go.viam.com/core/component/motor"
)

// Motor is an injected motor.
type Motor struct {
	motor.Motor
	SetPowerFunc          func(ctx context.Context, powerPct float64) error
	GoFunc                func(ctx context.Context, powerPct float64) error
	GoForFunc             func(ctx context.Context, rpm float64, rotations float64) error
	GoToFunc              func(ctx context.Context, rpm float64, position float64) error
	GoTillStopFunc        func(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error
	SetToZeroPositionFunc func(ctx context.Context, offset float64) error
	PositionFunc          func(ctx context.Context) (float64, error)
	PositionSupportedFunc func(ctx context.Context) (bool, error)
	OffFunc               func(ctx context.Context) error
	IsOnFunc              func(ctx context.Context) (bool, error)
}

// SetPower calls the injected Power or the real version.
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	if m.SetPowerFunc == nil {
		return m.Motor.SetPower(ctx, powerPct)
	}
	return m.SetPowerFunc(ctx, powerPct)
}

// Go calls the injected Go or the real version.
func (m *Motor) Go(ctx context.Context, powerPct float64) error {
	if m.GoFunc == nil {
		return m.Motor.Go(ctx, powerPct)
	}
	return m.GoFunc(ctx, powerPct)
}

// GoFor calls the injected GoFor or the real version.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(ctx, rpm, revolutions)
	}
	return m.GoForFunc(ctx, rpm, revolutions)
}

// GoTo calls the injected GoTo or the real version.
func (m *Motor) GoTo(ctx context.Context, rpm float64, position float64) error {
	if m.GoToFunc == nil {
		return m.Motor.GoTo(ctx, rpm, position)
	}
	return m.GoToFunc(ctx, rpm, position)
}

// GoTillStop calls the injected GoTillStop or the real version.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if m.GoTillStopFunc == nil {
		return m.Motor.GoTillStop(ctx, rpm, stopFunc)
	}
	return m.GoTillStopFunc(ctx, rpm, stopFunc)
}

// SetToZeroPosition calls the injected Zero or the real version.
func (m *Motor) SetToZeroPosition(ctx context.Context, offset float64) error {
	if m.SetToZeroPositionFunc == nil {
		return m.Motor.SetToZeroPosition(ctx, offset)
	}
	return m.SetToZeroPositionFunc(ctx, offset)
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

// Off calls the injected Off or the real version.
func (m *Motor) Off(ctx context.Context) error {
	if m.OffFunc == nil {
		return m.Motor.Off(ctx)
	}
	return m.OffFunc(ctx)
}

// IsOn calls the injected IsOn or the real version.
func (m *Motor) IsOn(ctx context.Context) (bool, error) {
	if m.IsOnFunc == nil {
		return m.Motor.IsOn(ctx)
	}
	return m.IsOnFunc(ctx)
}

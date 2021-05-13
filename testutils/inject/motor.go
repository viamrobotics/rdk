package inject

import (
	"context"

	"go.viam.com/core/board"
	pb "go.viam.com/core/proto/api/v1"
)

// Motor is an injected motor.
type Motor struct {
	board.Motor
	PowerFunc             func(ctx context.Context, powerPct float32) error
	GoFunc                func(ctx context.Context, d pb.DirectionRelative, powerPct float32) error
	GoForFunc             func(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error
	PositionFunc          func(ctx context.Context) (float64, error)
	PositionSupportedFunc func(ctx context.Context) (bool, error)
	OffFunc               func(ctx context.Context) error
	IsOnFunc              func(ctx context.Context) (bool, error)
}

// Power calls the injected Power or the real version.
func (m *Motor) Power(ctx context.Context, powerPct float32) error {
	if m.PowerFunc == nil {
		return m.Motor.Power(ctx, powerPct)
	}
	return m.PowerFunc(ctx, powerPct)
}

// Go calls the injected Go or the real version.
func (m *Motor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	if m.GoFunc == nil {
		return m.Motor.Go(ctx, d, powerPct)
	}
	return m.GoFunc(ctx, d, powerPct)
}

// GoFor calls the injected GoFor or the real version.
func (m *Motor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(ctx, d, rpm, revolutions)
	}
	return m.GoForFunc(ctx, d, rpm, revolutions)
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

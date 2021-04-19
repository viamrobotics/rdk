package inject

import (
	"context"

	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
)

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

func (m *Motor) Power(ctx context.Context, powerPct float32) error {
	if m.PowerFunc == nil {
		return m.Motor.Power(ctx, powerPct)
	}
	return m.PowerFunc(ctx, powerPct)
}

func (m *Motor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	if m.GoFunc == nil {
		return m.Motor.Go(ctx, d, powerPct)
	}
	return m.GoFunc(ctx, d, powerPct)
}

func (m *Motor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(ctx, d, rpm, revolutions)
	}
	return m.GoForFunc(ctx, d, rpm, revolutions)
}

func (m *Motor) Position(ctx context.Context) (float64, error) {
	if m.PositionFunc == nil {
		return m.Motor.Position(ctx)
	}
	return m.PositionFunc(ctx)
}

func (m *Motor) PositionSupported(ctx context.Context) (bool, error) {
	if m.PositionSupportedFunc == nil {
		return m.Motor.PositionSupported(ctx)
	}
	return m.PositionSupportedFunc(ctx)
}

func (m *Motor) Off(ctx context.Context) error {
	if m.OffFunc == nil {
		return m.Motor.Off(ctx)
	}
	return m.OffFunc(ctx)
}

func (m *Motor) IsOn(ctx context.Context) (bool, error) {
	if m.IsOnFunc == nil {
		return m.Motor.IsOn(ctx)
	}
	return m.IsOnFunc(ctx)
}

package inject

import (
	"context"

	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
)

type Motor struct {
	board.Motor
	ForceFunc             func(ctx context.Context, force byte) error
	GoFunc                func(ctx context.Context, d pb.DirectionRelative, force byte) error
	GoForFunc             func(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error
	PositionFunc          func(ctx context.Context) (float64, error)
	PositionSupportedFunc func(ctx context.Context) (bool, error)
	OffFunc               func(ctx context.Context) error
	IsOnFunc              func(ctx context.Context) (bool, error)
}

func (m *Motor) Force(ctx context.Context, force byte) error {
	if m.ForceFunc == nil {
		return m.Motor.Force(ctx, force)
	}
	return m.ForceFunc(ctx, force)
}

func (m *Motor) Go(ctx context.Context, d pb.DirectionRelative, force byte) error {
	if m.GoFunc == nil {
		return m.Motor.Go(ctx, d, force)
	}
	return m.GoFunc(ctx, d, force)
}

func (m *Motor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	if m.GoForFunc == nil {
		return m.Motor.GoFor(ctx, d, rpm, rotations)
	}
	return m.GoForFunc(ctx, d, rpm, rotations)
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

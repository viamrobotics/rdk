package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
)

// Base is an injected base.
type Base struct {
	base.LocalBase
	DoFunc           func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	MoveStraightFunc func(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error
	SpinFunc         func(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error
	GetWidthFunc     func(ctx context.Context) (int, error)
	StopFunc         func(ctx context.Context, extra map[string]interface{}) error
	IsMovingFunc     func(context.Context) (bool, error)
	CloseFunc        func(ctx context.Context) error
}

// MoveStraight calls the injected MoveStraight or the real version.
func (b *Base) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	if b.MoveStraightFunc == nil {
		return b.LocalBase.MoveStraight(ctx, distanceMm, mmPerSec, extra)
	}
	return b.MoveStraightFunc(ctx, distanceMm, mmPerSec, extra)
}

// Spin calls the injected Spin or the real version.
func (b *Base) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	if b.SpinFunc == nil {
		return b.LocalBase.Spin(ctx, angleDeg, degsPerSec, extra)
	}
	return b.SpinFunc(ctx, angleDeg, degsPerSec, extra)
}

// GetWidth calls the injected GetWidth or the real version.
func (b *Base) GetWidth(ctx context.Context) (int, error) {
	if b.GetWidthFunc == nil {
		return b.LocalBase.GetWidth(ctx)
	}
	return b.GetWidthFunc(ctx)
}

// Stop calls the injected Stop or the real version.
func (b *Base) Stop(ctx context.Context, extra map[string]interface{}) error {
	if b.StopFunc == nil {
		return b.LocalBase.Stop(ctx, extra)
	}
	return b.StopFunc(ctx, extra)
}

// IsMoving calls the injected IsMoving or the real version.
func (b *Base) IsMoving(ctx context.Context) (bool, error) {
	if b.IsMovingFunc == nil {
		return b.LocalBase.IsMoving(ctx)
	}
	return b.IsMovingFunc(ctx)
}

// Close calls the injected Close or the real version.
func (b *Base) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		return utils.TryClose(ctx, b.LocalBase)
	}
	return b.CloseFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (b *Base) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if b.DoFunc == nil {
		return b.LocalBase.DoCommand(ctx, cmd)
	}
	return b.DoFunc(ctx, cmd)
}

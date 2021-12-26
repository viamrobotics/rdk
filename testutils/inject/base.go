package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/base"
)

// Base is an injected base.
type Base struct {
	base.Base
	MoveStraightFunc func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error
	SpinFunc         func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error
	WidthMillisFunc  func(ctx context.Context) (int, error)
	StopFunc         func(ctx context.Context) error
	CloseFunc        func(ctx context.Context) error
}

// MoveStraight calls the injected MoveStraight or the real version.
func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	if b.MoveStraightFunc == nil {
		return b.Base.MoveStraight(ctx, distanceMillis, millisPerSec, block)
	}
	return b.MoveStraightFunc(ctx, distanceMillis, millisPerSec, block)
}

// Spin calls the injected Spin or the real version.
func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	if b.SpinFunc == nil {
		return b.Base.Spin(ctx, angleDeg, degsPerSec, block)
	}
	return b.SpinFunc(ctx, angleDeg, degsPerSec, block)
}

// WidthMillis calls the injected WidthMillis or the real version.
func (b *Base) WidthMillis(ctx context.Context) (int, error) {
	if b.WidthMillisFunc == nil {
		return b.Base.WidthMillis(ctx)
	}
	return b.WidthMillisFunc(ctx)
}

// Stop calls the injected Stop or the real version.
func (b *Base) Stop(ctx context.Context) error {
	if b.StopFunc == nil {
		return b.Base.Stop(ctx)
	}
	return b.StopFunc(ctx)
}

// Close calls the injected Close or the real version.
func (b *Base) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		return utils.TryClose(ctx, b.Base)
	}
	return b.CloseFunc(ctx)
}

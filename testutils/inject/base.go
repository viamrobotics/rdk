package inject

import (
	"context"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/utils"
)

type Base struct {
	api.Base
	MoveStraightFunc func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error)
	SpinFunc         func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error)
	WidthMillisFunc  func(ctx context.Context) (int, error)
	StopFunc         func(ctx context.Context) error
	CloseFunc        func() error
}

func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error) {
	if b.MoveStraightFunc == nil {
		return b.Base.MoveStraight(ctx, distanceMillis, millisPerSec, block)
	}
	return b.MoveStraightFunc(ctx, distanceMillis, millisPerSec, block)
}

func (b *Base) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
	if b.SpinFunc == nil {
		return b.Base.Spin(ctx, angleDeg, degsPerSec, block)
	}
	return b.SpinFunc(ctx, angleDeg, degsPerSec, block)
}

func (b *Base) WidthMillis(ctx context.Context) (int, error) {
	if b.WidthMillisFunc == nil {
		return b.Base.WidthMillis(ctx)
	}
	return b.WidthMillisFunc(ctx)
}

func (b *Base) Stop(ctx context.Context) error {
	if b.StopFunc == nil {
		return b.Base.Stop(ctx)
	}
	return b.StopFunc(ctx)
}

func (b *Base) Close() error {
	if b.CloseFunc == nil {
		return utils.TryClose(b.Base)
	}
	return b.CloseFunc()
}

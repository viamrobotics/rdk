package inject

import (
	"context"

	"go.viam.com/robotcore/base"
)

type Base struct {
	base.Device
	MoveStraightFunc func(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error
	SpinFunc         func(ctx context.Context, angleDeg float64, speed int, block bool) error
	WidthMillisFunc  func(ctx context.Context) (int, error)
	StopFunc         func(ctx context.Context) error
	CloseFunc        func(ctx context.Context) error
}

func (b *Base) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	if b.MoveStraightFunc == nil {
		return b.Device.MoveStraight(ctx, distanceMillis, millisPerSec, block)
	}
	return b.MoveStraightFunc(ctx, distanceMillis, millisPerSec, block)
}

func (b *Base) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {
	if b.SpinFunc == nil {
		return b.Device.Spin(ctx, angleDeg, speed, block)
	}
	return b.SpinFunc(ctx, angleDeg, speed, block)
}

func (b *Base) WidthMillis(ctx context.Context) (int, error) {
	if b.WidthMillisFunc == nil {
		return b.Device.WidthMillis(ctx)
	}
	return b.WidthMillisFunc(ctx)
}

func (b *Base) Stop(ctx context.Context) error {
	if b.StopFunc == nil {
		return b.Device.Stop(ctx)
	}
	return b.StopFunc(ctx)
}

func (b *Base) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		return b.Device.Close(ctx)
	}
	return b.CloseFunc(ctx)
}

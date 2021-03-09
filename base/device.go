package base

import "context"

type Device interface {
	MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error
	Spin(ctx context.Context, angleDeg float64, speed int, block bool) error
	Stop(ctx context.Context) error
	Close(ctx context.Context) error
	WidthMillis(ctx context.Context) (int, error)
}

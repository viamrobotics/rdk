package base

import "context"

type Device interface {
	MoveStraight(ctx context.Context, distanceMM int, mmPerSec float64, block bool) error
	Spin(ctx context.Context, angleDeg float64, speed int, block bool) error
	Stop(ctx context.Context) error
	Close(ctx context.Context) error
	Width(ctx context.Context) (float64, error)
}

package api

import (
	"context"
	"math"
)

type Base interface {
	MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error
	Spin(ctx context.Context, angleDeg float64, speed int, block bool) error
	Stop(ctx context.Context) error
	Close(ctx context.Context) error
	WidthMillis(ctx context.Context) (int, error)
}

type Move struct {
	DistanceMillis int
	AngleDeg       float64
	MillisPerSec   float64
	Block          bool
}

func DoMove(ctx context.Context, move Move, device Base) (float64, int, error) {
	if move.AngleDeg != 0 {
		// TODO(erh): speed is wrong
		if err := device.Spin(ctx, move.AngleDeg, int(move.MillisPerSec), move.Block); err != nil {
			// TODO(erd): Spin should report amount spun if errored
			return math.NaN(), 0, err
		}
	}

	if move.DistanceMillis != 0 {
		if err := device.MoveStraight(ctx, move.DistanceMillis, move.MillisPerSec, move.Block); err != nil {
			// TODO(erd): MoveStraight should report amount moved if errored
			return move.AngleDeg, 0, err
		}
	}

	return move.AngleDeg, move.DistanceMillis, nil
}

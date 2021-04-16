package api

import (
	"context"
)

type Base interface {
	MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error)
	Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error)
	Stop(ctx context.Context) error
	WidthMillis(ctx context.Context) (int, error)
}

type Move struct {
	DistanceMillis int
	MillisPerSec   float64
	AngleDeg       float64
	DegsPerSec     float64
	Block          bool
}

func DoMove(ctx context.Context, move Move, device Base) (float64, int, error) {
	var spunAmout float64
	if move.AngleDeg != 0 {
		spun, err := device.Spin(ctx, move.AngleDeg, move.DegsPerSec, move.Block)
		if err != nil {
			return spun, 0, err
		}
		spunAmout = spun
	}

	var movedAmount int
	if move.DistanceMillis != 0 {
		moved, err := device.MoveStraight(ctx, move.DistanceMillis, move.MillisPerSec, move.Block)
		if err != nil {
			return spunAmout, moved, err
		}
		movedAmount = moved
	}

	return spunAmout, movedAmount, nil
}

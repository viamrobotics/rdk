package base

import (
	"context"
	"math"
)

type Move struct {
	DistanceMM int
	AngleDeg   float64
	Speed      float64
	Block      bool
}

func DoMove(ctx context.Context, move Move, device Device) (float64, int, error) {
	if move.AngleDeg != 0 {
		// TODO(erh): speed is wrong
		if err := device.Spin(ctx, move.AngleDeg, int(move.Speed), move.Block); err != nil {
			// TODO(erd): Spin should report amount spun if errored
			return math.NaN(), 0, err
		}
	}

	if move.DistanceMM != 0 {
		if err := device.MoveStraight(ctx, move.DistanceMM, move.Speed, move.Block); err != nil {
			// TODO(erd): MoveStraight should report amount moved if errored
			return move.AngleDeg, 0, err
		}
	}

	return move.AngleDeg, move.DistanceMM, nil
}

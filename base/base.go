// Package base defines the base that a robot uses to move around.
package base

import (
	"context"
)

// A Base represents a physical base of a robot.
type Base interface {
	// MoveStraight moves the robot straight a given distance at a given speed. The method
	// can be requested to block until the move is complete.
	MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) (int, error)

	// MoveArc moves the robot in an arc a given distance at a given speed and angle. The method
	// can be requested to block until move is complete
	// Note: ramping affects when and how arc is performed, further improvements may be needed
	MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, angleDeg float64, block bool) (int, error)

	// Spin spins the robot by a given angle in degrees at a given speed. The method
	// can be requested to block until the move is complete.
	Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error)

	// Stop stops the base. It is assumed the base stops immediately.
	Stop(ctx context.Context) error

	// WidthMillis returns the width of the base.
	WidthMillis(ctx context.Context) (int, error)
}

// A Move describes instructions for a robot to spin followed by moving straight.
type Move struct {
	DistanceMillis int
	MillisPerSec   float64
	AngleDeg       float64
	DegsPerSec     float64
	Block          bool
}

// DoMove performs the given move on the given base.
func DoMove(ctx context.Context, move Move, base Base) (float64, int, error) {
	var spunAmout float64
	if move.AngleDeg != 0 {
		spun, err := base.Spin(ctx, move.AngleDeg, move.DegsPerSec, move.Block)
		if err != nil {
			return spun, 0, err
		}
		spunAmout = spun
	}

	var movedAmount int
	if move.DistanceMillis != 0 {
		moved, err := base.MoveStraight(ctx, move.DistanceMillis, move.MillisPerSec, move.Block)
		if err != nil {
			return spunAmout, moved, err
		}
		movedAmount = moved
	}

	return spunAmout, movedAmount, nil
}

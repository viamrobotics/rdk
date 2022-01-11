// Package base defines the base that a robot uses to move around.
package base

import (
	"context"
)

// A Base represents a physical base of a robot.
type Base interface {
	// MoveStraight moves the robot straight a given distance at a given speed. The method
	// can be requested to block until the move is complete. If a distance or speed of zero is given,
	// the base will stop.
	MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error

	// MoveArc moves the robot in an arc a given distance at a given speed and degs per second of movement.
	// The degs per sec represents the angular velocity the robot has during its movement. This function
	// can be requested to block until move is complete. If a distance of 0 is given the resultant motion
	// is a spin and if speed of 0 is given the base will stop.
	// Note: ramping affects when and how arc is performed, further improvements may be needed
	MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, degsPerSec float64, block bool) error

	// Spin spins the robot by a given angle in degrees at a given speed. The method can be requested
	// to block until the move is complete. If a speed of 0 the base will stop.
	Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error

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
func DoMove(ctx context.Context, move Move, base Base) error {
	if move.AngleDeg != 0 {
		err := base.Spin(ctx, move.AngleDeg, move.DegsPerSec, move.Block)
		if err != nil {
			return err
		}
	}

	if move.DistanceMillis != 0 {
		err := base.MoveStraight(ctx, move.DistanceMillis, move.MillisPerSec, move.Block)
		if err != nil {
			return err
		}
	}

	return nil
}

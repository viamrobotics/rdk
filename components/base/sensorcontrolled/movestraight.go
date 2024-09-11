// Package sensorcontrolled base implements a base with feedback control from a movement sensor
package sensorcontrolled

import (
	"context"
	"errors"
	"math"
	"time"

	geo "github.com/kellydunn/golang-geo"
)

const (
	slowDownDistGain      = .1
	maxSlowDownDist       = 100 // mm
	moveStraightErrTarget = 0   // mm
	headingGain           = 1.
)

// MoveStraight commands a base to move forward for the desired distanceMm at the given mmPerSec.
// When controls are enabled, MoveStraight calculates the required velocity to reach mmPerSec
// and the distanceMm goal. It then polls the provided velocity movement sensor and corrects any
// error between this calculated velocity and the actual velocity using a PID control loop.
// MoveStraight also monitors the position and stops the base when the goal distanceMm is reached.
// If a compass heading movement sensor is provided, MoveStraight will attempt to keep the heading
// of the base fixed in the original direction it was faced at the beginning of the MoveStraight call.
func (sb *sensorBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	ctx, done := sb.opMgr.New(ctx)
	defer done()

	// If a position movement sensor or controls are not configured, we cannot use this MoveStraight method.
	// Instead we need to use the MoveStraight method of the base that the sensorcontrolled base wraps.
	// If there is no valid velocity sensor, there won't be a controlLoopConfig.
	if sb.controlLoopConfig == nil {
		sb.logger.CWarnf(ctx,
			"control loop not configured, using base %s's MoveStraight",
			sb.controlledBase.Name().ShortName())
		if sb.loop != nil {
			sb.loop.Pause()
		}
		return sb.controlledBase.MoveStraight(ctx, distanceMm, mmPerSec, extra)
	}
	if sb.position == nil {
		sb.logger.CWarn(ctx,
			"controlling using linear velocity only, for increased accuracy add a position reporting sensor")

		// adjust inputs to ensure errDist is always positive to match the position based implementation
		if distanceMm < 0 {
			distanceMm = -distanceMm
			mmPerSec = -mmPerSec
		}
	}

	// check tuning status
	if err := sb.checkTuningStatus(); err != nil {
		return err
	}

	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
	}

	// pause and resume the loop to reset the control blocks.
	// This prevents any residual signals in the control loop from "kicking" the robot
	sb.loop.Pause()
	sb.loop.Resume()

	straightTimeEst := time.Duration(int(time.Second) * int(math.Abs(float64(distanceMm)/mmPerSec)))
	startTime := time.Now()
	timeOut := 5 * straightTimeEst
	if timeOut < 10*time.Second {
		timeOut = 10 * time.Second
	}

	// grab the initial heading for MoveStraight to clamp to. Will return 0 if no supporting sensors were configured.
	initialHeading, _, err := sb.headingFunc(ctx)
	if err != nil {
		return err
	}

	// initialize relevant parameters for moving straight
	slowDownDist := calcSlowDownDist(distanceMm)

	var initPos *geo.Point

	if sb.position != nil {
		initPos, _, err = sb.position.Position(ctx, nil)
		if err != nil {
			return err
		}
	}

	// this state is only used when no position sensor is configured
	prevTime := startTime
	currDistMm := 0.

	ticker := time.NewTicker(time.Duration(1000./sb.controlLoopConfig.Frequency) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// context.cancelled can happen due to UI being closed during MoveStraight.
			// Do not return context canceled errors, just log them
			if errors.Is(ctx.Err(), context.Canceled) {
				sb.logger.Warnf("Context cancelled during MoveStraight ", ctx.Err())
				return nil
			}
			return ctx.Err()
		case <-ticker.C:
			var errDist float64

			angVelDes, err := sb.calcHeadingControl(ctx, initialHeading)
			if err != nil {
				return err
			}

			if sb.position != nil {
				errDist, err = sb.calcPositionError(ctx, distanceMm, initPos)
				if err != nil {
					return err
				}
			} else {
				currTime := time.Now()
				vels, err := sb.velocities.LinearVelocity(ctx, nil)
				if err != nil {
					return err
				}
				deltaTime := currTime.Sub(prevTime).Seconds()
				// calculate the estimated change in position based on the latest velocity
				deltaPosMm := sign(mmPerSec) * vels.Y * deltaTime * 1000
				currDistMm += deltaPosMm
				errDist = float64(distanceMm) - currDistMm
				prevTime = currTime
			}

			if errDist < moveStraightErrTarget {
				return sb.Stop(ctx, nil)
			}

			linVelDes := calcLinVel(errDist, mmPerSec, slowDownDist)
			if err != nil {
				return err
			}

			// update velocity controller
			if err := sb.updateControlConfig(ctx, linVelDes/1000.0, angVelDes); err != nil {
				return err
			}

			// exit if the straight takes too long
			if time.Since(startTime) > timeOut {
				sb.logger.CWarn(ctx, "exceeded time for MoveStraight call, stopping base")
				return sb.Stop(ctx, nil)
			}
		}
	}
}

// calculate the desired angular velocity to correct the heading of the base.
func (sb *sensorBase) calcHeadingControl(ctx context.Context, initHeading float64) (float64, error) {
	currHeading, _, err := sb.headingFunc(ctx)
	if err != nil {
		return 0, err
	}

	headingErr := initHeading - currHeading
	headingErrWrapped := headingErr - (math.Floor((headingErr+180.)/(2*180.)))*2*180. // [-180;180)

	return headingErrWrapped * headingGain, nil
}

// calcPositionError calculates the current error in position.
// This results in the distance the base needs to travel to reach the goal.
func (sb *sensorBase) calcPositionError(ctx context.Context, distanceMm int, initPos *geo.Point) (float64, error) {
	pos, _, err := sb.position.Position(ctx, nil)
	if err != nil {
		return 0, err
	}

	// the currDist will always return as positive, so we need the goal distanceMm to be positive
	currDist := initPos.GreatCircleDistance(pos) * 1000000.
	return math.Abs(float64(distanceMm)) - currDist, nil
}

// calcLinVel computes the desired linear velocity based on how far the base is from reaching the goal.
func calcLinVel(errDist, mmPerSec, slowDownDist float64) float64 {
	// have the velocity slow down when appoaching the goal. Otherwise use the desired velocity
	linVel := errDist * mmPerSec / slowDownDist
	absMmPerSec := math.Abs(mmPerSec)
	if math.Abs(linVel) > absMmPerSec {
		return absMmPerSec * sign(linVel)
	}
	return linVel
}

// calcSlowDownDist computes the distance at which the MoveStraight call should begin to slow down.
// This helps to prevent overshoot when reaching the goal and reduces the jerk on the robot when the straight is complete.
func calcSlowDownDist(distanceMm int) float64 {
	slowDownDist := float64(distanceMm) * slowDownDistGain
	if math.Abs(slowDownDist) > maxSlowDownDist {
		return maxSlowDownDist * sign(float64(distanceMm))
	}
	return slowDownDist
}

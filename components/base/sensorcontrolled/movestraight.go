// Package sensorcontrolled base implements a base with feedback control from a movement sensor
package sensorcontrolled

import (
	"context"
	"math"
	"time"

	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
)

const (
	slowDownDistGain      = .1
	maxSlowDownDist       = 100 // mm
	moveStraightErrTarget = 20  // mm
	headingGain           = 1.
)

func (sb *sensorBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{},
) error {
	ctx, done := sb.opMgr.New(ctx)
	defer done()
	sb.setPolling(false)
	straightTimeEst := time.Duration(int(time.Second) * int(math.Abs(float64(distanceMm)/mmPerSec)))
	startTime := time.Now()
	timeOut := 5 * straightTimeEst
	if timeOut < 10*time.Second {
		timeOut = 10 * time.Second
	}

	if sb.position == nil || len(sb.conf.ControlParameters) == 0 {
		sb.logger.CWarnf(ctx, "Position reporting sensor not available, and no control loop is configured, using base %s MoveStraight", sb.controlledBase.Name().ShortName())
		sb.stopLoop()
		return sb.controlledBase.MoveStraight(ctx, distanceMm, mmPerSec, extra)
	}

	initialHeading, err := sb.headingFunc(ctx)
	if err != nil {
		return err
	}
	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
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

	ticker := time.NewTicker(time.Duration(1000./sb.controlLoopConfig.Frequency) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			var errDist float64

			angVelDes, err := sb.calcHeadingControl(ctx, initialHeading)
			if err != nil {
				return err
			}

			if sb.position != nil {
				errDist, err = calcPositionError(ctx, distanceMm, initPos, sb.position)
				if err != nil {
					return err
				}
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
				sb.logger.CWarn(ctx, "exceeded time for MoveStraightCall, stopping base")

				return sb.Stop(ctx, nil)
			}
		}
	}
}

// calculate the desired angular velocity to correct the heading of the base.
func (sb *sensorBase) calcHeadingControl(ctx context.Context, initHeading float64) (float64, error) {
	currHeading, err := sb.headingFunc(ctx)
	if err != nil {
		return 0, err
	}

	headingErr := initHeading - currHeading
	headingErrWrapped := headingErr - (math.Floor((headingErr+180.)/(2*180.)))*2*180.
	sb.logger.Info("Current heading: ", currHeading)
	sb.logger.Info("Initial heading: ", initHeading)
	sb.logger.Info("    Err heading: ", headingErrWrapped)

	return headingErrWrapped * headingGain, nil
}

// calcPositionError calculates the current error in position.
// This results in the distance the base needs to travel to reach the goal.
func calcPositionError(ctx context.Context, distanceMm int, initPos *geo.Point,
	position movementsensor.MovementSensor,
) (float64, error) {
	pos, _, err := position.Position(ctx, nil)
	if err != nil {
		return 0, err
	}

	currDist := initPos.GreatCircleDistance(pos) * 1000000.
	return float64(distanceMm) - currDist, nil
}

// calcLinVel computes the desired linear velocity based on how far the base is from reaching the goal.
func calcLinVel(errDist, mmPerSec, slowDownDist float64) float64 {
	// have the velocity slow down when appoaching the goal. Otherwise use the desired velocity
	linVel := errDist * mmPerSec / slowDownDist
	if linVel > mmPerSec {
		return mmPerSec
	}
	if linVel < -mmPerSec {
		return -mmPerSec
	}
	return linVel
}

// calcSlowDownDist computes the distance at which the MoveStraigh call should begin to slow down.
// This helps to prevent overshoot when reaching the goal and reduces the jerk on the robot when the straight is complete.
func calcSlowDownDist(distanceMm int) float64 {
	return math.Min(float64(distanceMm)*slowDownDistGain, maxSlowDownDist)
}

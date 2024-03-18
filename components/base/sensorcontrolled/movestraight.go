// Package sensorcontrolled base implements a base with feedback control from a movement sensor
package sensorcontrolled

import (
	"context"
	"errors"
	"math"
	"time"

	geo "github.com/kellydunn/golang-geo"

	rdkutils "go.viam.com/rdk/utils"
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

	if sb.position == nil || len(sb.controlLoopConfig.Blocks) == 0 {
		sb.logger.CWarnf(ctx,
			"Position reporting sensor not available, and no control loop is configured, using base %s MoveStraight",
			sb.controlledBase.Name().ShortName())
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
			// do not return context canceled errors, just log them
			if errors.Is(ctx.Err(), context.Canceled) {
				sb.logger.Error(ctx.Err())
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

// determineHeadingFunc determines which movement sensor endpoint should be used for control.
// The priority is Orientation -> Heading -> No heading control.
func (sb *sensorBase) determineHeadingFunc(ctx context.Context) {
	switch {
	case sb.orientation != nil:
		sb.headingFunc = func(ctx context.Context) (float64, error) {
			orient, err := sb.orientation.Orientation(ctx, nil)
			if err != nil {
				return 0, err
			}
			// this returns (-180-> 180)
			yaw := rdkutils.RadToDeg(orient.EulerAngles().Yaw)

			return yaw, nil
		}
	case sb.compassHeading != nil:
		sb.headingFunc = func(ctx context.Context) (float64, error) {
			compassHeading, err := sb.compassHeading.CompassHeading(ctx, nil)
			if err != nil {
				return 0, err
			}
			// make the compass heading (-180->180)
			if compassHeading > 180 {
				compassHeading -= 360
			}

			return compassHeading, nil
		}
	default:
		sb.logger.CInfof(ctx, "base %v cannot control heading, no heading related sensor given",
			sb.Name().ShortName())
		sb.headingFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
	}
}

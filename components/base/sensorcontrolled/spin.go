package sensorcontrolled

import (
	"context"
	"math"
	"time"

	rdkutils "go.viam.com/rdk/utils"
)

const (
	increment   = 0.01 // angle fraction multiplier to check
	oneTurn     = 360.0
	slowDownAng = 15. // angle from goal for spin to begin breaking
)

// Spin commands a base to turn about its center at an angular speed and for a specific angle.
func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	// if controls are not configured, use the underlying base's spin
	if len(sb.conf.ControlParameters) == 0 {
		sb.logger.CWarn(ctx, "control parameters not configured, switching to default spin")
		return sb.controlledBase.Spin(ctx, angleDeg, degsPerSec, extra)
	}
	if sb.orientation == nil {
		sb.logger.CWarn(ctx, "orientation movement sensor not configured, switching to default spin")
		return sb.controlledBase.Spin(ctx, angleDeg, degsPerSec, extra)
	}

	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
	}
	sb.setPolling(true)

	orientation, err := sb.orientation.Orientation(ctx, nil)
	if err != nil {
		return err
	}
	prevAngle := rdkutils.RadToDeg(orientation.EulerAngles().Yaw)
	angErr := 0.
	prevMovedAng := 0.
	ticker := time.NewTicker(time.Duration(1000./sb.controlLoopConfig.Frequency) * time.Millisecond)
	defer ticker.Stop()

	// timeout duration is a multiplier times the expected time to perform a movement
	spinTimeEst := time.Duration(int(time.Second) * int(math.Abs(angleDeg/degsPerSec)))
	startTime := time.Now()
	timeOut := 5 * spinTimeEst
	if timeOut < 10*time.Second {
		timeOut = 10 * time.Second
	}

	for {
		// check if we want to poll the sensor at all
		// other API calls set this to false so that this for loop stops

		if !sb.isPolling() {
			ticker.Stop()
			sb.logger.CWarn(ctx, "Spin call interrupted by another running api")
			return nil
		}

		if err := ctx.Err(); err != nil {
			ticker.Stop()
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			orientation, err := sb.orientation.Orientation(ctx, nil)
			if err != nil {
				return err
			}

			currYaw := rdkutils.RadToDeg(orientation.EulerAngles().Yaw)
			angErr, prevMovedAng = getAngError(currYaw, prevAngle, prevMovedAng, angleDeg)
			prevAngle = currYaw

			if math.Abs(angErr) < boundCheckTarget {
				return sb.Stop(ctx, nil)
			}
			angVel := calcAngVel(angErr, degsPerSec)

			if err := sb.updateControlConfig(ctx, 0, angVel); err != nil {
				return err
			}

			if time.Since(startTime) > timeOut {
				sb.logger.CWarn(ctx, "exceeded time for Spin call, stopping base")
				if err := sb.Stop(ctx, nil); err != nil {
					return err
				}
				return nil
			}
		}
	}
}

// calcAngVel computes the desired angular velocity based on how far the base is from reaching the goal.
func calcAngVel(angErr, degsPerSec float64) float64 {
	// have the velocity slow down when appoaching the goal. Otherwise use the desired velocity
	angVel := angErr * degsPerSec / slowDownAng
	if angVel > degsPerSec {
		return degsPerSec
	}
	if angVel < -degsPerSec {
		return -degsPerSec
	}
	return angVel
}

func getAngError(currYaw, prevAngle, prevMovedAng, desiredAngle float64) (float64, float64) {
	// use initial angle to get the current angle the spin has moved
	angMoved := getMovedAng(prevAngle, currYaw, prevMovedAng)

	// compute the error
	errAng := (desiredAngle - angMoved)

	return errAng, angMoved
}

// getMovedAng tracks how much the angle has moved between each sensor update.
// This allows us to convert a bounded angle(0 to 360 or -180 to 180) into the raw angle traveled.
func getMovedAng(prevAngle, currAngle, angMoved float64) float64 {
	// the angle changed from 180 to -180. this means we are spinning in the negative direction
	if currAngle-prevAngle < -300 {
		return angMoved + currAngle - prevAngle + 360
	}
	// the angle changed from -180 to 180
	if currAngle-prevAngle > 300 {
		return angMoved + currAngle - prevAngle - 360
	}
	// add the change in angle to the position
	return angMoved + currAngle - prevAngle
}

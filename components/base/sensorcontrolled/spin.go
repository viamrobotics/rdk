package sensorcontrolled

import (
	"context"
	"errors"
	"math"
	"time"

	"go.viam.com/rdk/components/movementsensor"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	increment        = 0.01 // angle fraction multiplier to check
	oneTurn          = 360.0
	maxSlowDownAng   = 30. // maximum angle from goal for spin to begin breaking
	slowDownAngGain  = 0.1 // Use the final 10% of the requested spin to slow down
	boundCheckTarget = 1.  // error threshold for spin
)

// Spin commands a base to turn about its center at an angular speed and for a specific angle.
func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	// if controls are not configured, we cannot use this spin method. Instead we need to use the spin method
	// of the base that the sensorBase wraps.
	if len(sb.controlLoopConfig.Blocks) == 0 {
		sb.logger.CWarnf(ctx, "control parameters not configured, using %v's spin method", sb.controlledBase.Name().ShortName())
		return sb.controlledBase.Spin(ctx, angleDeg, degsPerSec, extra)
	}
	if sb.orientation == nil {
		sb.logger.CWarn(ctx, "orientation movement sensor not configured,using %v's spin method", sb.controlledBase.Name().ShortName())
		return sb.controlledBase.Spin(ctx, angleDeg, degsPerSec, extra)
	}

	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
	}
	ctx, done := sb.opMgr.New(ctx)
	defer done()
	sb.setPolling(true)

	orientation, err := sb.orientation.Orientation(ctx, nil)
	if err != nil {
		return err
	}
	prevAngle := rdkutils.RadToDeg(orientation.EulerAngles().Yaw)
	angErr := 0.
	prevMovedAng := 0.

	// to keep the signs simple, ensure degsPerSec is positive and let angleDeg handle the direction of the spin
	if degsPerSec < 0 {
		angleDeg = -angleDeg
		degsPerSec = -degsPerSec
	}
	slowDownAng := calcSlowDownAng(angleDeg)

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
			// do not return context canceled errors, just log them
			if errors.Is(ctx.Err(), context.Canceled) {
				sb.logger.Error(ctx.Err())
				return nil
			}
			return err
		case <-ticker.C:

			currYaw, err := getYawInDeg(ctx, sb.orientation)
			if err != nil {
				return err
			}
			angErr, prevMovedAng = getAngError(currYaw, prevAngle, prevMovedAng, angleDeg)

			if math.Abs(angErr) < boundCheckTarget {
				return sb.Stop(ctx, nil)
			}
			angVel := calcAngVel(angErr, degsPerSec, slowDownAng)

			if err := sb.updateControlConfig(ctx, 0, angVel); err != nil {
				return err
			}

			// track the previous angle to compute how much we moved with each iteration
			prevAngle = currYaw

			// check if the duration of the spin exceeds the expected length of the spin
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

// getYawInDeg gets the yaw from a movement sensor that supports orientation and returns it in degrees.
func getYawInDeg(ctx context.Context, orientation movementsensor.MovementSensor) (float64, error) {
	ori, err := orientation.Orientation(ctx, nil)
	if err != nil {
		return 0., err
	}

	return rdkutils.RadToDeg(ori.EulerAngles().Yaw), nil
}

// calcSlowDownAng computes the angle at which the spin should begin to slow down.
// This helps to prevent overshoot when reaching the goal and reduces the jerk on the robot when the spin is complete.
// This term should always be positive.
func calcSlowDownAng(angleDeg float64) float64 {
	return math.Min(math.Abs(angleDeg)*slowDownAngGain, maxSlowDownAng)
}

// calcAngVel computes the desired angular velocity based on how far the base is from reaching the goal.
func calcAngVel(angErr, degsPerSec, slowDownAng float64) float64 {
	// have the velocity slow down when appoaching the goal. Otherwise use the desired velocity
	angVel := angErr * degsPerSec / slowDownAng
	if math.Abs(angVel) > degsPerSec {
		return degsPerSec * sign(angVel)
	}
	return angVel
}

// getAngError computes the current distance the spin has moved and returns how much further the base must move to reach the goal.
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

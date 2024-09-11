package sensorcontrolled

import (
	"context"
	"errors"
	"math"
	"time"
)

const (
	increment        = 0.01 // angle fraction multiplier to check
	oneTurn          = 360.0
	maxSlowDownAng   = 30. // maximum angle from goal for spin to begin breaking
	slowDownAngGain  = 0.1 // Use the final 10% of the requested spin to slow down
	boundCheckTarget = 1.  // error threshold for spin
)

// Spin commands a base to turn about its center at an angular speed and for a specific angle.
// When controls are enabled, Spin polls the provided orientation movement sensor and corrects
// any error between the desired degsPerSec and the actual degsPerSec using a PID control loop.
// Spin also monitors the angleDeg and stops the base when the goal angle is reached.
func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	sb.opMgr.CancelRunning(ctx)
	ctx, done := sb.opMgr.New(ctx)
	defer done()

	// If an orientation movement sensor or controls are not configured, we cannot use this Spin method.
	// Instead we need to use the Spin method of the base that the sensorBase wraps.
	// If there is no valid velocity sensor, there won't be a controlLoopConfig.
	if sb.controlLoopConfig == nil {
		sb.logger.CWarnf(ctx, "control parameters not configured, using %v's Spin method", sb.controlledBase.Name().ShortName())
		return sb.controlledBase.Spin(ctx, angleDeg, degsPerSec, extra)
	}

	// check tuning status
	if err := sb.checkTuningStatus(); err != nil {
		return err
	}

	prevAngle, hasOrientation, err := sb.headingFunc(ctx)
	if err != nil {
		return err
	}

	if !hasOrientation {
		sb.logger.CWarn(ctx,
			"controlling using angular velocity only, for increased accuracy add an orientation or compass heading reporting sensor")
	}

	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
	}

	// pause and resume the loop to reset the control blocks
	// This prevents any residual signals in the control loop from "kicking" the robot
	sb.loop.Pause()
	sb.loop.Resume()
	var angErr, angMoved float64

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
	prevTime := startTime

	for {
		if err := ctx.Err(); err != nil {
			ticker.Stop()
			return err
		}

		select {
		case <-ctx.Done():
			// context.cancelled can happen due to UI being closed during Spin.
			// Do not return context canceled errors, just log them
			if errors.Is(ctx.Err(), context.Canceled) {
				sb.logger.Warn("Context cancelled during Spin ", ctx.Err())
				return nil
			}
			return err
		case <-ticker.C:

			if hasOrientation {
				currYaw, _, err := sb.headingFunc(ctx)
				if err != nil {
					return err
				}
				// use initial angle to get the current angle the spin has moved
				angMoved = getMovedAng(prevAngle, currYaw, angMoved)

				// track the previous angle to compute how much we moved with each iteration
				prevAngle = currYaw
			} else {
				currTime := time.Now()
				angVels, err := sb.velocities.AngularVelocity(ctx, nil)
				if err != nil {
					return err
				}
				deltaTime := currTime.Sub(prevTime).Seconds()
				// calculate the estimated change in angle based on the latest angular velocity
				deltaAngDeg := angVels.Z * deltaTime
				angMoved += deltaAngDeg

				// track time for the velocity integration
				prevTime = currTime
			}

			// compute the error
			angErr = (angleDeg - angMoved)

			if math.Abs(angErr) < boundCheckTarget {
				return sb.Stop(ctx, nil)
			}
			angVel := calcAngVel(angErr, degsPerSec, slowDownAng)

			if err := sb.updateControlConfig(ctx, 0, angVel); err != nil {
				return err
			}

			// check if the duration of the spin exceeds the expected length of the spin
			if time.Since(startTime) > timeOut {
				sb.logger.CWarn(ctx, "exceeded time for Spin call, stopping base")
				return sb.Stop(ctx, nil)
			}
		}
	}
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

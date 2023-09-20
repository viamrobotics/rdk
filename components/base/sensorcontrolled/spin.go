package sensorcontrolled

import (
	"context"
	"math"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	rdkutils "go.viam.com/rdk/utils"
)

func (sb *sensorBase) stopSpinWithSensor(
	ctx context.Context, angleDeg, degsPerSec float64,
) error {
	// imu readings are limited from 0 -> 360
	startYaw, err := getCurrentYaw(sb.orientation)
	if err != nil {
		return err
	}

	targetYaw, dir, _ := findSpinParams(angleDeg, degsPerSec, startYaw)

	errBound := boundCheckTarget

	// reset error counter for imu reading errors
	errCounter := 0

	// timeout duration is a multiplier times the expected time to perform a movement
	spinTimeEst := time.Duration(int(time.Second) * int(math.Abs(angleDeg/degsPerSec)))
	startTime := time.Now()
	timeOut := 5 * spinTimeEst
	if timeOut < 10*time.Second {
		timeOut = 10 * time.Second
	}

	sb.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ticker := time.NewTicker(yawPollTime)
		defer ticker.Stop()

		for {
			// check if we want to poll the sensor at all
			// other API calls set this to false so that this for loop stops
			if !sb.isPolling() {
				ticker.Stop()
			}

			if err := ctx.Err(); err != nil {
				ticker.Stop()
				return
			}

			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				// imu readings are limited from 0 -> 360
				currYaw, err := getCurrentYaw(sb.orientation)
				if err != nil {
					errCounter++
					if errCounter > 100 {
						sb.logger.Error(errors.Wrap(
							err,
							"imu sensor unreachable, 100 error counts when trying to read yaw angle, stopping base"))
						if err = sb.Stop(ctx, nil); err != nil {
							return
						}
					}
					continue
				}
				errCounter = 0 // reset reading error count to zero if we are successfully reading again

				atTarget, overShot, minTravel := getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg, errBound)

				if sensorDebug {
					sb.logger.Debugf("minTravel %t, atTarget %t, overshot %t", minTravel, atTarget, overShot)
					sb.logger.Debugf("angleDeg %.2f, increment %.2f", // , fullTurns %d",
						angleDeg, increment) // , fullTurns)
					sb.logger.Debugf("currYaw %.2f, startYaw %.2f, targetYaw %.2f",
						currYaw, startYaw, targetYaw)
				}

				// poll the sensor for the current error in angle
				// check if we've overshot our target by the errTarget value
				// check if we've travelled at all
				if minTravel && (atTarget || overShot) {
					if sensorDebug {
						sb.logger.Debugf(
							"stopping base with errAngle:%.2f, overshot? %t",
							math.Abs(targetYaw-currYaw), overShot)
					}

					if err := sb.Stop(ctx, nil); err != nil {
						return
					}
				}

				if time.Since(startTime) > timeOut {
					sb.logger.Warn("exceeded time for Spin call, stopping base")
					if err := sb.Stop(ctx, nil); err != nil {
						return
					}
					return
				}
			}
		}
	}, sb.activeBackgroundWorkers.Done)
	return nil
}

func getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg, errorBound float64) (atTarget, overShot, minTravel bool) {
	atTarget = math.Abs(targetYaw-currYaw) < errorBound
	overShot = hasOverShot(currYaw, startYaw, targetYaw, dir)
	travelIncrement := math.Abs(angleDeg * increment)
	// // check the case where we're asking for a 360 degree turn, this results in a zero travelIncrement
	if math.Abs(angleDeg) < 360 {
		minTravel = math.Abs(currYaw-startYaw) > travelIncrement
	} else {
		minTravel = false
	}
	return atTarget, overShot, minTravel
}

func getCurrentYaw(ms movementsensor.MovementSensor,
) (float64, error) {
	ctx, done := context.WithCancel(context.Background())
	defer done()
	orientation, err := ms.Orientation(ctx, nil)
	if err != nil {
		return math.NaN(), err
	}
	// add Pi to make the computation for overshoot simpler
	// turns imus from -180 -> 180 to a 0 -> 360 range
	return addAnglesInDomain(rdkutils.RadToDeg(orientation.EulerAngles().Yaw), 0), nil
}

func addAnglesInDomain(target, current float64) float64 {
	angle := target + current
	// reduce the angle
	angle = math.Mod(angle, oneTurn)

	// force it to be the positive remainder, so that 0 <= angle < 360
	angle = math.Mod(angle+oneTurn, oneTurn)
	return angle
}

func findSpinParams(angleDeg, degsPerSec, currYaw float64) (float64, float64, int) {
	dir := 1.0
	if math.Signbit(degsPerSec) != math.Signbit(angleDeg) {
		// both positive or both negative -> counterclockwise spin call
		// counterclockwise spin calls add angles
		// the signs being different --> clockwise spin call
		// clockwise spin calls subtract angles
		dir = -1.0
	}
	targetYaw := addAnglesInDomain(angleDeg, currYaw)
	fullTurns := int(math.Abs(angleDeg)) / oneTurn

	return targetYaw, dir, fullTurns
}

// this function does not wrap around 360 degrees currently.
func angleBetween(current, bound1, bound2 float64) bool {
	if bound2 > bound1 {
		inBewtween := current >= bound1 && current < bound2
		return inBewtween
	}
	inBewteen := current > bound2 && current <= bound1
	return inBewteen
}

func hasOverShot(angle, start, target, dir float64) bool {
	switch {
	case dir == -1 && start > target: // clockwise
		// for cases with a quadrant switch from 1 <-> 4
		// check if the current angle is in the regions before the
		// target and after the start
		over := angleBetween(angle, target, 0) || angleBetween(angle, 360, start)
		return over
	case dir == -1 && target > start:
		// the overshoot range is the inside range between the start and target
		return angleBetween(angle, target, start)
	case dir == 1 && start > target: // counterclockwise
		// for cases with a quadrant switch from 1 <-> 4
		// check if the current angle is not in the regions after the
		// target and before the start
		over := !angleBetween(angle, 0, target) && !angleBetween(angle, start, 360)
		return over
	default:
		// the overshoot range is the range of angles outside the start and target ranges
		return !angleBetween(angle, start, target)
	}
}

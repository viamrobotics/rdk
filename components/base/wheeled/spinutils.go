package wheeled

import (
	"context"
	"math"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	rdkutils "go.viam.com/rdk/utils"
)

func (base *wheeledBase) spinWithMovementSensor(
	ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{},
) error {
	startYaw, err := base.getCurrentYaw(ctx, base.orientation, extra)
	if err != nil {
		return err
	} // from 0 -> 360

	targetYaw, dir, fullTurns := findSpinParams(angleDeg, degsPerSec, startYaw)
	turnCount := 0
	errCounter := 0

	base.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ticker := time.NewTicker(yawPollTime)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			default:
			}
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				// imu readings are limited from 0 -> 360
				currYaw, err := base.getCurrentYaw(ctx, base.orientation, extra)
				if err != nil {
					errCounter++
					if errCounter > 100 {
						base.logger.Error(errors.Wrap(
							err, "imu sensor unreachable, 100 error counts when trying to read yaw angle"))
						return
					}
					return
				}
				errCounter = 0 // reset reading error count to zero if we are successfully reading again

				atTarget, overShot, minTravel := getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg)

				// if the imu yaw reading is close to 360, we are nearing a full turn,
				// so we adjust the current reading by 360 * the number of turns we've done
				if atTarget && (fullTurns > 0) {
					turnCount++
					overShot = false
					minTravel = true
				}

				if sensorDebug {
					base.logger.Debugf("minTravel %t, atTarget %t, overshot %t", minTravel, atTarget, overShot)
					base.logger.Debugf("angleDeg %.2f, increment %.2f, turnCount %d, fullTurns %d",
						angleDeg, increment, turnCount, fullTurns)
					base.logger.Debugf("currYaw %.2f, startYaw %.2f, targetYaw %.2f", currYaw, startYaw, targetYaw)
				}

				// poll the sensor for the current error in angle
				// check if we've overshot our target by the errTarget value
				// check if we've travelled at all
				if fullTurns == 0 {
					if (atTarget && minTravel) || (overShot && minTravel) {
						ticker.Stop()
						if err := base.Stop(ctx, nil); err != nil {
							return
						}
						if sensorDebug {
							base.logger.Debugf(
								"stopping base with errAngle:%.2f, overshot? %t",
								math.Abs(targetYaw-currYaw), overShot)
						}
					}
				} else {
					if (turnCount >= fullTurns) && atTarget {
						ticker.Stop()
						if err := base.Stop(ctx, nil); err != nil {
							return
						}
						if sensorDebug {
							base.logger.Debugf(
								"stopping base with errAngle:%.2f, overshot? %t, fullTurns %d, turnCount %d",
								math.Abs(targetYaw-currYaw), overShot, fullTurns, turnCount)
						}
					}
				}
			}
		}
	}, base.activeBackgroundWorkers.Done)
	return nil
}

func (base *wheeledBase) stopSensors() {
	if len(base.allSensors) != 0 {
		base.sensorDone()
		base.sensorCtx, base.sensorDone = context.WithCancel(context.Background())
	}
}

func getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg float64) (atTarget, overShot, minTravel bool) {
	atTarget = math.Abs(targetYaw-currYaw) < errTarget
	overShot = hasOverShot(currYaw, startYaw, targetYaw, dir)
	minTravel = math.Abs(currYaw-startYaw) > math.Abs(angleDeg*increment)
	return atTarget, overShot, minTravel
}

func (base *wheeledBase) getCurrentYaw(ctx context.Context, ms movementsensor.MovementSensor, extra map[string]interface{},
) (float64, error) {
	ctx, done := context.WithCancel(ctx)
	defer done()
	orientation, err := ms.Orientation(ctx, extra)
	if err != nil {
		return 0, errors.Wrap(
			err, "error getting orientation from sensor, spin will proceed without sensor feedback",
		)
	}
	// Add Pi  to make the computation for overshoot simpler
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
	targetYaw := addAnglesInDomain(angleDeg, currYaw)
	dir := 1.0
	if math.Signbit(degsPerSec) != math.Signbit(angleDeg) {
		// both positive or both negative -> counterclockwise spin call
		// counterclockwise spin calls add allowable angles
		// the signs being different --> clockwise spin call
		// cloxkwise spin calls subtract allowable angles
		dir = -1
	}
	fullTurns := int(math.Abs(angleDeg)) / int(oneTurn)

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
	case dir == -1 && start > target:
		// for cases with a quadrant switch from 1 -> 4
		// check if the current angle is in the regions before the
		// target and after the start
		over := angleBetween(angle, target, 0) || angleBetween(angle, 360, start)
		return over
	case dir == -1 && target > start:
		// the overshoot range is the inside range between the start and target
		return angleBetween(angle, target, start)
	case dir == 1 && start > target:
		// for cases with a quadrant switch from 1 <-> 4
		// check if the current angle is not in the regions after the
		// target and before the start
		over := !angleBetween(angle, 0, target) && !angleBetween(angle, start, 360)
		return over
	default:
		// the overshoot range is the outside range between the start and target
		return !angleBetween(angle, start, target)
	}
}

package sensorcontrolled

import (
	"context"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	increment = 0.01 // angle fraction multiplier to check
	oneTurn   = 360.0
)

// Spin commands a base to turn about its center at a angular speed and for a specific angle.
func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	sb.stopLoop()
	if int(angleDeg) >= 360 {
		sb.setPolling(false)
		sb.logger.CWarn(ctx, "feedback for spin calls over 360 not supported yet, spinning without sensor")
		return sb.controlledBase.Spin(ctx, angleDeg, degsPerSec, nil)
	}
	ctx, done := sb.opMgr.New(ctx)
	defer done()
	// check if a sensor context has been started
	if sb.sensorLoopDone != nil {
		sb.sensorLoopDone()
	}

	sb.setPolling(true)
	// start a sensor context for the sensor loop based on the longstanding base
	// creator context
	var sensorCtx context.Context
	sensorCtx, sb.sensorLoopDone = context.WithCancel(context.Background())
	if err := sb.stopSpinWithSensor(sensorCtx, angleDeg, degsPerSec); err != nil {
		return err
	}

	// starts a goroutine from within wheeled base's runAll function to run motors in the background
	if err := sb.startRunningMotors(ctx, angleDeg, degsPerSec); err != nil {
		return err
	}

	// isPolling returns true when a Spin call is in progress, which is not a success condition for our control loop
	baseStopped := func(ctx context.Context) (bool, error) {
		polling := sb.isPolling()
		return !polling, nil
	}

	return sb.opMgr.WaitForSuccess(
		ctx,
		yawPollTime,
		baseStopped,
	)
}

func (sb *sensorBase) startRunningMotors(ctx context.Context, angleDeg, degsPerSec float64) error {
	if math.Signbit(angleDeg) != math.Signbit(degsPerSec) {
		degsPerSec *= -1
	}
	return sb.controlledBase.SetVelocity(ctx,
		r3.Vector{X: 0, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 0, Z: degsPerSec}, nil)
}

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
						sb.logger.CError(ctx, errors.Wrap(
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
					sb.logger.CDebugf(ctx, "minTravel %t, atTarget %t, overshot %t", minTravel, atTarget, overShot)
					sb.logger.CDebugf(ctx, "angleDeg %.2f", angleDeg)
					sb.logger.CDebugf(ctx,
						"currYaw %.2f, startYaw %.2f, targetYaw %.2f",
						currYaw, startYaw, targetYaw)
				}

				// poll the sensor for the current error in angle
				// check if we've overshot our target by the errTarget value
				// check if we've travelled at all
				if minTravel && (atTarget || overShot) {
					if sensorDebug {
						sb.logger.CDebugf(ctx,
							"stopping base with errAngle:%.2f, overshot? %t",
							math.Abs(targetYaw-currYaw), overShot)
					}

					if err := sb.Stop(ctx, nil); err != nil {
						return
					}
				}

				if time.Since(startTime) > timeOut {
					sb.logger.CWarn(ctx, "exceeded time for Spin call, stopping base")
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
	// when the individual motors of a base have a control loop set up, they move slightly in the wrong direction before
	// spinning in the right direction. when this happens, hasOverShot thinks the goal has already been passed, so there
	// needs to be a small window (boundCheckOverShot) that forces overShot to be false until the motors have started
	// moving past startYaw in the correct direction
	if dir == 1 {
		offset := 0.0
		// if subtracting boundCheckOverShot from startYaw results in a negative, we need to un-wrap currentYaw around
		// 360 so that the comparison between the two is of the correct scale.
		// For example, if startYaw is 5, starYaw-boundCheckOverShot is -5. if currYaw were also at -5, it would be
		// reported as 355, so we must subract 360 to make currentYaw actually be -5 in order for the comparison to
		// yield the expected result
		if startYaw-boundCheckOverShot < 0 {
			offset = -360.0
		}
		if currYaw < startYaw && currYaw+offset > startYaw-boundCheckOverShot {
			overShot = false
		}
	} else {
		offset := 0.0
		// if adding boundCheckOverShot to startYaw results in a value over 360, we need to un-wrap currentYaw around
		// 360 so that the comparison between the two is of the correct scale
		if startYaw+boundCheckOverShot > 0 {
			offset = 360.0
		}
		if currYaw > startYaw && currYaw+offset < startYaw+boundCheckOverShot {
			overShot = false
		}
	}
	travelIncrement := math.Abs(angleDeg * increment)
	// check the case where we're asking for a 360 degree turn, this results in a zero travelIncrement
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

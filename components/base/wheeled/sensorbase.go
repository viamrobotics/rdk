package wheeled

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	yawPollTime      = 5 * time.Millisecond
	boundCheckTurn   = 2.0
	boundCheckTarget = 5.0
	oneTurn          = 360.0
	increment        = 0.01
	sensorDebug      = true
)

type sensorBase struct {
	resource.Named
	resource.AlwaysRebuild
	logger golog.Logger

	activeBackgroundWorkers sync.WaitGroup
	base                    base.Base
	baseCtx                 context.Context

	sensorMu      sync.Mutex
	sensorDone    func()
	sensorPolling bool

	opMgr operation.SingleOperationManager

	allSensors  []movementsensor.MovementSensor
	orientation movementsensor.MovementSensor
}

func attachSensorsToBase(
	ctx context.Context,
	base *wheeledBase,
	deps resource.Dependencies,
	ms []string,
	logger golog.Logger,
) (base.Base, error) {
	// use base's creator context as the long standing context for the sensor loop
	sb := &sensorBase{base: base, logger: logger, baseCtx: ctx, Named: base.Name().AsNamed()}

	var oriMsName string
	for _, msName := range ms {
		ms, err := movementsensor.FromDependencies(deps, msName)
		if err != nil {
			return nil, errors.Wrapf(err, "no movement_sensor namesd (%s)", msName)
		}
		sb.allSensors = append(sb.allSensors, ms)
		props, err := ms.Properties(ctx, nil)
		if props.OrientationSupported && err == nil {
			sb.orientation = ms
			oriMsName = msName
		}
	}

	sb.logger.Infof("using sensor %s as orientation sensor for base", oriMsName)

	return sb, nil
}

// setPolling determines whether we want the sensor loop to run and stop the base with sensor feedback
// should be set to false everywhere except when sensor feedback should be polled
// currently when a orientation reporting sensor is used in Spin.
func (sb *sensorBase) setPolling(isActive bool) {
	sb.sensorMu.Lock()
	sb.sensorPolling = isActive
	sb.sensorMu.Unlock()
}

// isPolling gets whether the base is actively polling a sensor.
func (sb *sensorBase) isPolling() bool {
	sb.sensorMu.Lock()
	defer sb.sensorMu.Unlock()
	return sb.sensorPolling
}

// Spin commands a base to turn about its center at a angular speed and for a specific angle.
func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	ctx, done := sb.opMgr.New(ctx)
	defer done()

	// imu readings are limited from 0 -> 360
	startYaw, err := getCurrentYaw(ctx, sb.orientation)
	if err != nil {
		return err
	}

	_, dir, fullTurns := findSpinParams(angleDeg, degsPerSec, startYaw)
	if fullTurns > 0 {
		sb.startRunningMotors(ctx, oneTurn*float64(fullTurns)*dir, degsPerSec)
	}

	// check if a sensor context has been started
	if sb.sensorDone != nil {
		sb.sensorDone()
	}

	// starts a goroutine with wheeled base's runAll function running in the backgorund
	sb.startRunningMotors(ctx, angleDeg, degsPerSec)

	// start a sensor context for the sensor loop based on the longstanding base
	// creator context
	var sensorCtx context.Context
	sensorCtx, sb.sensorDone = context.WithCancel(sb.baseCtx)

	sb.setPolling(true)
	return sb.stopSpinWithSensor(sensorCtx, angleDeg-oneTurn*float64(fullTurns), degsPerSec)
}

func (sb *sensorBase) startRunningMotors(ctx context.Context, angleDeg, degsPerSec float64) {
	wb := sb.base.(*wheeledBase)

	rpm, _ := wb.spinMath(angleDeg, degsPerSec)

	sb.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		if err := wb.runAll(ctx, -rpm, 0, rpm, 0); err != nil {
			sb.logger.Error(err)
		}
	}, sb.activeBackgroundWorkers.Done)

}

func (sb *sensorBase) stopSpinWithSensor(
	ctx context.Context, angleDeg, degsPerSec float64,
) error {
	// imu readings are limited from 0 -> 360
	startYaw, err := getCurrentYaw(ctx, sb.orientation)
	if err != nil {
		return err
	}

	targetYaw, dir, _ := findSpinParams(angleDeg, degsPerSec, startYaw)

	errBound := boundCheckTarget
	// checkTurns := math.Abs(angleDeg) >= oneTurn && fullTurns > 0

	// reset error counter for imu reading errors
	errCounter := 0

	startTime := time.Now()
	// timeout duration is a multiplier times the expected time to perform a movement
	spinTimeEst := time.Duration(int(time.Second) * int(math.Abs(angleDeg/degsPerSec)))
	timeOut := 5 * spinTimeEst
	if 5*spinTimeEst < 30*time.Second {
		timeOut = 30 * time.Second
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
				return
			}

			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:

				// imu readings are limited from 0 -> 360
				currYaw, err := getCurrentYaw(ctx, sb.orientation)
				if err != nil {
					errCounter++
					if errCounter > 100 {
						sb.logger.Error(errors.Wrap(
							err, "imu sensor unreachable, 100 error counts when trying to read yaw angle, stopping base"))
						if err := sb.Stop(ctx, nil); err != nil {
							sb.logger.Error(err)
						}
						return
					}
					continue
				}

				// if the imu yaw reading is close to 360, we are near a full turn,
				// so we adjust the current reading by 360 * the number of turns we've done
				// if rdkutils.Float64AlmostEqual(currYaw, oneTurn, boundCheckTurn) && checkTurns {
				// 	fullTurns--
				// 	continue
				// 	// if fullTurns < 0 {
				// 	// 	fullTurns = 0
				// 	// }

				// }

				errCounter = 0 // reset reading error count to zero if we are successfully reading again

				atTarget, overShot, minTravel := getTurnState(currYaw, startYaw, targetYaw, dir, angleDeg, errBound)

				if sensorDebug {
					sb.logger.Debugf("minTravel %t, atTarget %t, overshot %t", minTravel, atTarget, overShot)
					sb.logger.Debugf("angleDeg %.2f, increment %.2f", //, fullTurns %d",
						angleDeg, increment) //, fullTurns)
					sb.logger.Debugf("currYaw %.2f, startYaw %.2f, targetYaw %.2f",
						currYaw, startYaw, targetYaw)
				}

				// poll the sensor for the current error in angle
				// check if we've overshot our target by the errTarget value
				// check if we've travelled at all

				// if fullTurns == 0 {
				if minTravel && (atTarget || overShot) {
					if err := sb.Stop(ctx, nil); err != nil {
						return
					}
					if sensorDebug {
						sb.logger.Debugf(
							"stopping base with errAngle:%.2f, overshot? %t",
							math.Abs(targetYaw-currYaw), overShot)
					}
				}
				// }

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
	// if rdkutils.Float64AlmostEqual(travelIncrement, 0.0, 0.001) {
	// 	travelIncrement = boundCheckTarget
	// }
	if angleDeg < 360 {
		minTravel = math.Abs(currYaw-startYaw) > travelIncrement
	} else {
		minTravel = false
	}
	return atTarget, overShot, minTravel
}

func getCurrentYaw(ctx context.Context, ms movementsensor.MovementSensor,
) (float64, error) {
	ctx, done := context.WithCancel(ctx)
	defer done()
	orientation, err := ms.Orientation(ctx, nil)
	if err != nil {
		return 0, err
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
	targetYaw := addAnglesInDomain(angleDeg, currYaw)
	dir := 1.0
	if math.Signbit(degsPerSec) != math.Signbit(angleDeg) {
		// both positive or both negative -> counterclockwise spin call
		// counterclockwise spin calls add angles
		// the signs being different --> clockwise spin call
		// clockwise spin calls subtract angles
		dir = -1
	}
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

func (sb *sensorBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{},
) error {
	ctx, done := sb.opMgr.New(ctx)
	defer done()
	sb.setPolling(false)
	return sb.base.MoveStraight(ctx, distanceMm, mmPerSec, extra)
}

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.base.SetVelocity(ctx, linear, angular, extra)
}

func (sb *sensorBase) SetPower(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.base.SetPower(ctx, linear, angular, extra)
}

func (sb *sensorBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.base.Stop(ctx, extra)
}

func (sb *sensorBase) IsMoving(ctx context.Context) (bool, error) {
	return sb.base.IsMoving(ctx)
}

func (sb *sensorBase) Close(ctx context.Context) error {
	sb.activeBackgroundWorkers.Wait()

	// check if a sensor context has been started
	if sb.sensorDone != nil {
		sb.sensorDone()
	}

	return sb.Stop(ctx, nil)
}

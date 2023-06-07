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
	sensorDebug      = false
)

type sensorBase struct {
	resource.Named
	resource.AlwaysRebuild // TODO (rh) implement reconfigure
	logger                 golog.Logger

	activeBackgroundWorkers sync.WaitGroup
	wBase                   base.Base // the inherited wheeled base
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
	sb *sensorBase,
	deps resource.Dependencies,
	ms []string,
) error {
	// use base's creator context as the long standing context for the sensor loop
	var oriMsName string
	for _, msName := range ms {
		ms, err := movementsensor.FromDependencies(deps, msName)
		if err != nil {
			return errors.Wrapf(err, "no movement_sensor namesd (%s)", msName)
		}
		sb.allSensors = append(sb.allSensors, ms)
		props, err := ms.Properties(ctx, nil)
		if props.OrientationSupported && err == nil {
			sb.orientation = ms
			oriMsName = msName
		}
	}

	sb.logger.Infof("using sensor %s as orientation sensor for base", oriMsName)

	return nil
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

	sb.logger.Infof("angleDeg %v, degsPerSec %v", angleDeg, degsPerSec)
	switch {
	case angleDeg >= 360:
		sb.setPolling(false)
		sb.logger.Warn("feedback for spin calls over 360 not supported yet, spinning without sensor")
		return sb.wBase.Spin(ctx, angleDeg, degsPerSec, nil)
	default:
		// check if a sensor context has been started
		if sb.sensorDone != nil {
			sb.sensorDone()
		}

		sb.setPolling(true)
		// start a sensor context for the sensor loop based on the longstanding base
		// creator context
		var sensorCtx context.Context
		sensorCtx, sb.sensorDone = context.WithCancel(sb.baseCtx)
		if err := sb.stopSpinWithSensor(sensorCtx, angleDeg, degsPerSec); err != nil {
			return err
		}

		// starts a goroutine from within wheeled base's runAll function to run motors in the background
		return sb.startRunningMotors(ctx, angleDeg, degsPerSec)
	}
}

func (sb *sensorBase) startRunningMotors(ctx context.Context, angleDeg, degsPerSec float64) error {
	wb := sb.wBase.(*wheeledBase)
	rpm, _ := wb.spinMath(angleDeg, degsPerSec)
	err := wb.runAll(ctx, -rpm, 0, rpm, 0)
	return err
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

	// reset error counter for imu reading errors
	errCounter := 0

	startTime := time.Now()
	// timeout duration is a multiplier times the expected time to perform a movement
	spinTimeEst := time.Duration(int(time.Second) * int(math.Abs(angleDeg/degsPerSec)))
	timeOut := 5 * spinTimeEst
	if 5*spinTimeEst < 10*time.Second {
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
					if err := sb.Stop(ctx, nil); err != nil {
						return
					}
					if sensorDebug {
						sb.logger.Debugf(
							"stopping base with errAngle:%.2f, overshot? %t",
							math.Abs(targetYaw-currYaw), overShot)
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
	dir := 1.0
	if math.Signbit(degsPerSec) != math.Signbit(angleDeg) {
		// both positive or both negative -> counterclockwise spin call
		// counterclockwise spin calls add angles
		// the signs being different --> clockwise spin call
		// clockwise spin calls subtract angles
		dir = -1.0
	}
	targetYaw := addAnglesInDomain(angleDeg*dir, currYaw)
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
	return sb.wBase.MoveStraight(ctx, distanceMm, mmPerSec, extra)
}

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.wBase.SetVelocity(ctx, linear, angular, extra)
}

func (sb *sensorBase) SetPower(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.wBase.SetPower(ctx, linear, angular, extra)
}

func (sb *sensorBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	sb.logger.Info("stop called")
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.wBase.Stop(ctx, extra)
}

func (sb *sensorBase) IsMoving(ctx context.Context) (bool, error) {
	return sb.wBase.IsMoving(ctx)
}

func (sb *sensorBase) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	return sb.wBase.Properties(ctx, extra)
}

func (sb *sensorBase) Close(ctx context.Context) error {
	sb.activeBackgroundWorkers.Wait()
	sb.logger.Infof("Close called")

	// check if a sensor context has been started
	if sb.sensorDone != nil {
		sb.sensorDone()
	}

	return sb.Stop(ctx, nil)
}

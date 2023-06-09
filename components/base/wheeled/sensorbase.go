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
	// resource.AlwaysRebuild // TODO (rh) implement reconfigure
	logger golog.Logger
	mu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
	wBase                   base.Base // the inherited wheeled base

	sensorLoopMu      sync.Mutex
	sensorLoopDone    func()
	sensorLoopPolling bool

	opMgr operation.SingleOperationManager

	allSensors  []movementsensor.MovementSensor
	orientation movementsensor.MovementSensor
}

func (sb *sensorBase) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	if len(sb.allSensors) != len(newConf.MovementSensor) {
		for _, name := range newConf.MovementSensor {
			ms, err := movementsensor.FromDependencies(deps, name)
			if err != nil {
				return errors.Wrapf(err, "no movement sensor named (%s)", name)
			}
			sb.allSensors = append(sb.allSensors, ms)
		}
	} else {
		// Compare each element of the slices
		for i := range sb.allSensors {
			if sb.allSensors[i].Name().String() != newConf.MovementSensor[i] {
				for _, name := range newConf.MovementSensor {
					ms, err := movementsensor.FromDependencies(deps, name)
					if err != nil {
						return errors.Wrapf(err, "no movement sensor named (%s)", name)
					}
					sb.allSensors[i] = ms
				}
				break
			}
		}
	}

	var oriMsName string
	for _, ms := range sb.allSensors {
		props, err := ms.Properties(context.Background(), nil)
		if props.OrientationSupported && err == nil {
			sb.orientation = ms
			oriMsName = ms.Name().ShortName()
		}
	}

	sb.logger.Infof("using sensor %s as orientation sensor for base", oriMsName)
	return nil
}

// setPolling determines whether we want the sensor loop to run and stop the base with sensor feedback
// should be set to false everywhere except when sensor feedback should be polled
// currently when a orientation reporting sensor is used in Spin.
func (sb *sensorBase) setPolling(isActive bool) {
	sb.sensorLoopMu.Lock()
	defer sb.sensorLoopMu.Unlock()
	sb.sensorLoopPolling = isActive
}

// isPolling gets whether the base is actively polling a sensor.
func (sb *sensorBase) isPolling() bool {
	sb.sensorLoopMu.Lock()
	defer sb.sensorLoopMu.Unlock()
	return sb.sensorLoopPolling
}

// Spin commands a base to turn about its center at a angular speed and for a specific angle.
func (sb *sensorBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {

	sb.logger.Infof("angleDeg %v, degsPerSec %v", angleDeg, degsPerSec)
	switch {
	case int(angleDeg) >= 360:
		sb.setPolling(false)
		sb.logger.Warn("feedback for spin calls over 360 not supported yet, spinning without sensor")
		return sb.wBase.Spin(ctx, angleDeg, degsPerSec, nil)
	default:
		ctx, done := sb.opMgr.New(ctx)
		defer done()
		// check if a sensor context has been started
		if sb.sensorLoopDone != nil {
			sb.sensorLoopDone()
		}

		sb.logger.Infof("staring sensor loop")
		sb.setPolling(true)
		// start a sensor context for the sensor loop based on the longstanding base
		// creator context
		var sensorCtx context.Context
		sensorCtx, sb.sensorLoopDone = context.WithCancel(context.Background())
		if err := sb.stopSpinWithSensor(sensorCtx, angleDeg, degsPerSec); err != nil {
			return err
		}

		// // starts a goroutine from within wheeled base's runAll function to run motors in the background
		if err := sb.startRunningMotors(ctx, angleDeg, degsPerSec); err != nil {
			return err
		}

		wb := sb.wBase.(*wheeledBase)
		motor := wb.allMotors[0]
		sb.opMgr.WaitTillNotPowered(ctx, 500*time.Millisecond, motor,
			func(context.Context, map[string]interface{}) error {
				return nil
			},
		)
		return nil
	}
}

func (sb *sensorBase) startRunningMotors(ctx context.Context, angleDeg, degsPerSec float64) error {
	if math.Signbit(angleDeg) != math.Signbit(degsPerSec) {
		degsPerSec *= -1
	}
	return sb.wBase.SetVelocity(ctx,
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

			if err = ctx.Err(); err != nil {
				sb.logger.Info("context cancelled")
				ticker.Stop()
				return
			}

			select {
			case <-ctx.Done():
				sb.logger.Info("context done")
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
					sb.logger.Infof("minTravel %t, atTarget %t, overshot %t", minTravel, atTarget, overShot)
					sb.logger.Infof("angleDeg %.2f, increment %.2f", // , fullTurns %d",
						angleDeg, increment) // , fullTurns)
					sb.logger.Infof("currYaw %.2f, startYaw %.2f, targetYaw %.2f",
						currYaw, startYaw, targetYaw)
				}

				// poll the sensor for the current error in angle
				// check if we've overshot our target by the errTarget value
				// check if we've travelled at all
				if minTravel && (atTarget || overShot) {
					// if sensorDebug {
					sb.logger.Infof(
						"stopping base with errAngle:%.2f, overshot? %t",
						math.Abs(targetYaw-currYaw), overShot)
					// }

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

			// if utils.SelectContextOrWait(ctx, timeOut) {
			// 	sb.Stop(ctx, nil)
			// 	sb.logger.Infof("closing context with time")
			// }
		}

	}, sb.activeBackgroundWorkers.Done)
	return err
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
	if err := sb.Stop(ctx, nil); err != nil {
		return err
	}
	// check if a sensor context is still alive
	if sb.sensorLoopDone != nil {
		sb.sensorLoopDone()
	}

	sb.activeBackgroundWorkers.Wait()
	return nil
}

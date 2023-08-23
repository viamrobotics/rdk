// Package sensorcontrolled base implements a base with feedback control from a movement sensor
package sensorcontrolled

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
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	yawPollTime        = 5 * time.Millisecond
	velocitiesPollTime = 5 * time.Millisecond
	boundCheckTurn     = 2.0
	boundCheckTarget   = 5.0
	oneTurn            = 360.0
	increment          = 0.01
	sensorDebug        = false
)

var (
	// Model is the name of the sensor_controlled model of a base component.
	Model           = resource.DefaultModelFamily.WithModel("sensor-controlled")
	errNoGoodSensor = errors.New("no appropriate sensor for orientaiton or velocity feedback")
)

// Config configures a sencor controlled base.
type Config struct {
	MovementSensor []string `json:"movement_sensor"`
	Base           string   `json:"base"`
}

// Validate validates all parts of the sensor controlled base config.
func (cfg *Config) Validate(path string) ([]string, error) {
	deps := []string{}
	if len(cfg.MovementSensor) == 0 {
		return nil, utils.NewConfigValidationError(path, errors.New("need at least one movement sensor for base"))
	}

	deps = append(deps, cfg.MovementSensor...)
	if cfg.Base == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "base")
	}

	deps = append(deps, cfg.Base)
	return deps, nil
}

type sensorBase struct {
	resource.Named
	logger golog.Logger
	mu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
	controlledBase          base.Base // the inherited wheeled base

	sensorLoopMu      sync.Mutex
	sensorLoopDone    func()
	sensorLoopPolling bool

	opMgr *operation.SingleOperationManager

	allSensors  []movementsensor.MovementSensor
	orientation movementsensor.MovementSensor
	velocities  movementsensor.MovementSensor
}

func init() {
	resource.RegisterComponent(
		base.API,
		Model,
		resource.Registration[base.Base, *Config]{Constructor: createSensorBase})
}

func createSensorBase(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (base.Base, error) {
	sb := &sensorBase{
		logger: logger,
		Named:  conf.ResourceName().AsNamed(),
		opMgr:  operation.NewSingleOperationManager(),
	}

	if err := sb.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return sb, nil
}

func (sb *sensorBase) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	// reset all sensors
	sb.allSensors = nil
	sb.velocities = nil
	sb.orientation = nil
	sb.controlledBase = nil

	for _, name := range newConf.MovementSensor {
		ms, err := movementsensor.FromDependencies(deps, name)
		if err != nil {
			return errors.Wrapf(err, "no movement sensor named (%s)", name)
		}
		sb.allSensors = append(sb.allSensors, ms)
	}

	for _, ms := range sb.allSensors {
		props, err := ms.Properties(context.Background(), nil)
		if err == nil && props.OrientationSupported {
			// return first sensor that does not error that satisfies the properties wanted
			sb.orientation = ms
			sb.logger.Infof("using sensor %s as orientation sensor for base", sb.orientation.Name().ShortName())
			break
		}
	}

	for _, ms := range sb.allSensors {
		props, err := ms.Properties(context.Background(), nil)
		if err == nil && props.AngularVelocitySupported && props.LinearVelocitySupported {
			// return first sensor that does not error that satisfies the properties wanted
			sb.velocities = ms
			sb.logger.Infof("using sensor %s as velocity sensor for base", sb.velocities.Name().ShortName())
			break
		}
	}

	if sb.orientation == nil && sb.velocities == nil {
		return errNoGoodSensor
	}

	sb.controlledBase, err = base.FromDependencies(deps, newConf.Base)
	if err != nil {
		return errors.Wrapf(err, "no base named (%s)", newConf.Base)
	}

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
	if int(angleDeg) >= 360 {
		sb.setPolling(false)
		sb.logger.Warn("feedback for spin calls over 360 not supported yet, spinning without sensor")
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

	// IsMoving returns true when moving, which is not a success condition for our control loop
	baseStopped := func(ctx context.Context) (bool, error) {
		moving, err := sb.IsMoving(ctx)
		return !moving, err
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

func (sb *sensorBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{},
) error {
	ctx, done := sb.opMgr.New(ctx)
	defer done()
	sb.setPolling(false)
	return sb.controlledBase.MoveStraight(ctx, distanceMm, mmPerSec, extra)
}

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	// check if a sensor context has been started
	if sb.sensorLoopDone != nil {
		sb.sensorLoopDone()
	}

	sb.setPolling(true)
	// start a sensor context for the sensor loop based on the longstanding base
	// creator context, and add a timeout for the context
	timeOut := 10 * time.Second
	var sensorCtx context.Context
	sensorCtx, sb.sensorLoopDone = context.WithTimeout(context.Background(), timeOut)

	if sb.velocities != nil {
		sb.logger.Warn("not using sensor for SetVelocityfeedback, this feature will be implemented soon")
		// TODO RSDK-3695 implement control loop here instead of placeholder sensor pllling function
		sb.pollsensors(sensorCtx, extra)
		return errors.New(
			"setvelocity with sensor feedback not currently implemented, remove movement sensor reporting linear and angular velocity ")
	}
	return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
}

func (sb *sensorBase) pollsensors(ctx context.Context, extra map[string]interface{}) {
	sb.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		ticker := time.NewTicker(velocitiesPollTime)
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
				return
			case <-ticker.C:
				linvel, err := sb.velocities.LinearVelocity(ctx, extra)
				if err != nil {
					sb.logger.Error(err)
					return
				}

				angvel, err := sb.velocities.AngularVelocity(ctx, extra)
				if err != nil {
					sb.logger.Error(err)
					return
				}

				if sensorDebug {
					sb.logger.Infof("sensor readings: linear: %#v, angular %#v", linvel, angvel)
				}
			}
		}
	}, sb.activeBackgroundWorkers.Done)
}

func (sb *sensorBase) SetPower(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.controlledBase.SetPower(ctx, linear, angular, extra)
}

func (sb *sensorBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	sb.opMgr.CancelRunning(ctx)
	sb.setPolling(false)
	return sb.controlledBase.Stop(ctx, extra)
}

func (sb *sensorBase) IsMoving(ctx context.Context) (bool, error) {
	return sb.controlledBase.IsMoving(ctx)
}

func (sb *sensorBase) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	return sb.controlledBase.Properties(ctx, extra)
}

func (sb *sensorBase) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return sb.controlledBase.Geometries(ctx, extra)
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

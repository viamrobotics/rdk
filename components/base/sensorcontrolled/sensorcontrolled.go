// Package sensorcontrolled base implements a base with feedback control from a movement sensor
package sensorcontrolled

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	yawPollTime           = 5 * time.Millisecond
	velocitiesPollTime    = 5 * time.Millisecond
	boundCheckTarget      = 5.0
	sensorDebug           = false
	typeLinVel            = "linear_velocity"
	typeAngVel            = "angular_velocity"
	slowDownDistGain      = .1
	maxSlowDownDist       = 100 // mm
	moveStraightErrTarget = 20  // mm
	headingGain           = 1.
)

var (
	// Model is the name of the sensor_controlled model of a base component.
	model           = resource.DefaultModelFamily.WithModel("sensor-controlled")
	errNoGoodSensor = errors.New("no appropriate sensor for orientation or velocity feedback")
)

// Config configures a sensor controlled base.
type Config struct {
	MovementSensor    []string            `json:"movement_sensor"`
	Base              string              `json:"base"`
	ControlParameters []control.PIDConfig `json:"control_parameters,omitempty"`
}

// Validate validates all parts of the sensor controlled base config.
func (cfg *Config) Validate(path string) ([]string, error) {
	deps := []string{}
	if len(cfg.MovementSensor) == 0 {
		return nil, resource.NewConfigValidationError(path, errors.New("need at least one movement sensor for base"))
	}

	deps = append(deps, cfg.MovementSensor...)
	if cfg.Base == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "base")
	}

	deps = append(deps, cfg.Base)
	return deps, nil
}

type sensorBase struct {
	resource.Named
	conf   *Config
	logger logging.Logger
	mu     sync.Mutex

	activeBackgroundWorkers sync.WaitGroup
	controlledBase          base.Base // the inherited wheeled base

	sensorLoopMu      sync.Mutex
	sensorLoopDone    func()
	sensorLoopPolling bool

	opMgr *operation.SingleOperationManager

	allSensors     []movementsensor.MovementSensor
	orientation    movementsensor.MovementSensor
	velocities     movementsensor.MovementSensor
	position       movementsensor.MovementSensor
	compassHeading movementsensor.MovementSensor
	headingFunc    func(ctx context.Context) (float64, error)

	controlLoopConfig control.Config
	blockNames        map[string][]string
	loop              *control.Loop
}

func init() {
	resource.RegisterComponent(
		base.API,
		model,
		resource.Registration[base.Base, *Config]{Constructor: createSensorBase})
}

func createSensorBase(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
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
	sb.conf = newConf
	if err != nil {
		return err
	}

	sb.stopLoop()

	sb.mu.Lock()
	defer sb.mu.Unlock()

	// reset all sensors
	sb.allSensors = nil
	sb.velocities = nil
	sb.orientation = nil
	sb.compassHeading = nil
	sb.position = nil
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
			sb.logger.CInfof(ctx, "using sensor %s as orientation sensor for base", sb.orientation.Name().ShortName())
			break
		}
	}

	for _, ms := range sb.allSensors {
		props, err := ms.Properties(context.Background(), nil)
		if err == nil && props.AngularVelocitySupported && props.LinearVelocitySupported {
			// return first sensor that does not error that satisfies the properties wanted
			sb.velocities = ms
			sb.logger.CInfof(ctx, "using sensor %s as velocity sensor for base", sb.velocities.Name().ShortName())
			break
		}
	}

	for _, ms := range sb.allSensors {
		props, err := ms.Properties(context.Background(), nil)
		if err == nil && props.PositionSupported {
			// return first sensor that does not error that satisfies the properties wanted
			sb.position = ms
			sb.logger.CInfof(ctx, "using sensor %s as position sensor for base", sb.position.Name().ShortName())
			break
		}
	}

	for _, ms := range sb.allSensors {
		props, err := ms.Properties(context.Background(), nil)
		if err == nil && props.CompassHeadingSupported {
			// return first sensor that does not error that satisfies the properties wanted
			sb.compassHeading = ms
			sb.logger.CInfof(ctx, "using sensor %s as compassHeading sensor for base", sb.compassHeading.Name().ShortName())
			break
		}
	}

	if sb.orientation == nil && sb.velocities == nil {
		return errNoGoodSensor
	}
	switch {
	case sb.orientation != nil:
		sb.headingFunc = func(ctx context.Context) (float64, error) {
			orient, err := sb.orientation.Orientation(ctx, nil)
			if err != nil {
				return 0, err
			}
			// this returns (-180-> 180)
			yaw := rdkutils.RadToDeg(orient.EulerAngles().Yaw)

			return yaw, nil
		}
	case sb.compassHeading != nil:
		sb.headingFunc = func(ctx context.Context) (float64, error) {
			compassHeading, err := sb.compassHeading.CompassHeading(ctx, nil)
			if err != nil {
				return 0, err
			}
			// make the compass heading (-180->180)
			if compassHeading > 180 {
				compassHeading = compassHeading - 360
			}

			return compassHeading, nil
		}
	default:
		sb.logger.CInfof(ctx, "base %v cannot control heading, no heading related sensor given",
			sb.Name().ShortName())
		sb.headingFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
	}

	sb.controlledBase, err = base.FromDependencies(deps, newConf.Base)
	if err != nil {
		return errors.Wrapf(err, "no base named (%s)", newConf.Base)
	}

	if sb.velocities != nil && len(newConf.ControlParameters) != 0 {
		// assign linear and angular PID correctly based on the given type
		var linear, angular control.PIDConfig
		for _, c := range newConf.ControlParameters {
			switch c.Type {
			case typeLinVel:
				linear = c
			case typeAngVel:
				angular = c
			default:
				sb.logger.Warn("control_parameters type must be 'linear_velocity' or 'angular_velocity'")
			}
		}

		// unlock the mutex before setting up the control loop so that the motors
		// are not locked, and can run if any auto-tuning is necessary
		sb.mu.Unlock()
		if err := sb.setupControlLoop(linear, angular); err != nil {
			sb.mu.Lock()
			return err
		}
		// relock the mutex after setting up the control loop since there is still a  defer unlock
		sb.mu.Lock()
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

func (sb *sensorBase) MoveStraight(
	ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{},
) error {
	ctx, done := sb.opMgr.New(ctx)
	defer done()
	sb.setPolling(false)
	straightTimeEst := time.Duration(int(time.Second) * int(math.Abs(float64(distanceMm)/mmPerSec)))
	startTime := time.Now()
	timeOut := 5 * straightTimeEst
	if timeOut < 10*time.Second {
		timeOut = 10 * time.Second
	}

	if sb.position == nil || len(sb.conf.ControlParameters) == 0 {
		sb.logger.CWarnf(ctx, "Position reporting sensor not available, and no control loop is configured, using base %s MoveStraight", sb.controlledBase.Name().ShortName())
		sb.stopLoop()
		return sb.controlledBase.MoveStraight(ctx, distanceMm, mmPerSec, extra)
	}

	initialHeading, err := sb.headingFunc(ctx)
	if err != nil {
		return err
	}
	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
	}

	// initialize relevant parameters for moving straight
	slowDownDist := calcSlowDownDist(distanceMm)
	var initPos *geo.Point

	if sb.position != nil {
		initPos, _, err = sb.position.Position(ctx, nil)
		if err != nil {
			return err
		}
	}

	ticker := time.NewTicker(time.Duration(1000./sb.controlLoopConfig.Frequency) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			var errDist float64

			angVelDes, err := sb.calcHeadingControl(ctx, initialHeading)
			if err != nil {
				return err
			}

			if sb.position != nil {
				errDist, err = calcPositionError(ctx, distanceMm, initPos, sb.position)
				if err != nil {
					return err
				}
			}

			if errDist < moveStraightErrTarget {
				return sb.Stop(ctx, nil)
			}

			linVelDes := calcLinVel(errDist, mmPerSec, slowDownDist)
			if err != nil {
				return err
			}

			// update velocity controller
			if err := sb.updateControlConfig(ctx, linVelDes/1000.0, angVelDes); err != nil {
				return err
			}

			// exit if the straight takes too long
			if time.Since(startTime) > timeOut {
				sb.logger.CWarn(ctx, "exceeded time for MoveStraightCall, stopping base")

				return sb.Stop(ctx, nil)
			}
		}
	}
}

// calculate the desired angular velocity to correct the heading of the base.
func (sb *sensorBase) calcHeadingControl(ctx context.Context, initHeading float64) (float64, error) {
	currHeading, err := sb.headingFunc(ctx)
	if err != nil {
		return 0, err
	}

	headingErr := initHeading - currHeading
	headingErrWrapped := headingErr - (math.Floor((headingErr+180.)/(2*180.)))*2*180.
	sb.logger.Info("Current heading: ", currHeading)
	sb.logger.Info("Initial heading: ", initHeading)
	sb.logger.Info("    Err heading: ", headingErrWrapped)

	return headingErrWrapped * headingGain, nil
}

// calcPositionError calculates the current error in position.
// This results in the distance the base needs to travel to reach the goal.
func calcPositionError(ctx context.Context, distanceMm int, initPos *geo.Point,
	position movementsensor.MovementSensor,
) (float64, error) {
	pos, _, err := position.Position(ctx, nil)
	if err != nil {
		return 0, err
	}

	currDist := initPos.GreatCircleDistance(pos) * 1000000.
	return float64(distanceMm) - currDist, nil
}

// calcLinVel computes the desired linear velocity based on how far the base is from reaching the goal.
func calcLinVel(errDist, mmPerSec, slowDownDist float64) float64 {
	// have the velocity slow down when appoaching the goal. Otherwise use the desired velocity
	linVel := errDist * mmPerSec / slowDownDist
	if linVel > mmPerSec {
		return mmPerSec
	}
	if linVel < -mmPerSec {
		return -mmPerSec
	}
	return linVel
}

// calcSlowDownDist computes the distance at which the MoveStraigh call should begin to slow down.
// This helps to prevent overshoot when reaching the goal and reduces the jerk on the robot when the straight is complete.
func calcSlowDownDist(distanceMm int) float64 {
	return math.Min(float64(distanceMm)*slowDownDistGain, maxSlowDownDist)
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
	if sb.sensorLoopDone != nil {
		sb.sensorLoopDone()
	}
	sb.stopLoop()
	return sb.controlledBase.Stop(ctx, extra)
}

func (sb *sensorBase) stopLoop() {
	if sb.loop != nil {
		sb.loop.Stop()
		sb.loop = nil
	}
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

// Package sensorcontrolled base implements a base with feedback control from a movement sensor
package sensorcontrolled

import (
	"context"
	"sync"
	"time"

	"github.com/golang/geo/r3"
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
	yawPollTime        = 5 * time.Millisecond
	velocitiesPollTime = 5 * time.Millisecond
	sensorDebug        = false
	typeLinVel         = "linear_velocity"
	typeAngVel         = "angular_velocity"
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

	opMgr *operation.SingleOperationManager

	allSensors []movementsensor.MovementSensor
	velocities movementsensor.MovementSensor
	position   movementsensor.MovementSensor
	// headingFunc returns the current angle between (-180,180) and whether Spin is supported
	headingFunc func(ctx context.Context) (float64, bool, error)

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

	if sb.loop != nil {
		sb.loop.Stop()
		sb.loop = nil
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	// reset all sensors
	sb.allSensors = nil
	sb.velocities = nil
	var orientation movementsensor.MovementSensor
	var compassHeading movementsensor.MovementSensor
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
			orientation = ms
			sb.logger.CInfof(ctx, "using sensor %s as orientation sensor for base", orientation.Name().ShortName())
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
			compassHeading = ms
			sb.logger.CInfof(ctx, "using sensor %s as compassHeading sensor for base", compassHeading.Name().ShortName())
			break
		}
	}
	sb.determineHeadingFunc(ctx, orientation, compassHeading)

	if orientation == nil && sb.velocities == nil {
		return errNoGoodSensor
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

func (sb *sensorBase) SetPower(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	if sb.loop != nil {
		sb.loop.Pause()
	}
	return sb.controlledBase.SetPower(ctx, linear, angular, extra)
}

func (sb *sensorBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	sb.opMgr.CancelRunning(ctx)
	if sb.loop != nil {
		sb.loop.Pause()
	}
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
	if sb.loop != nil {
		sb.loop.Stop()
		sb.loop = nil
	}

	sb.activeBackgroundWorkers.Wait()
	return nil
}

// determineHeadingFunc determines which movement sensor endpoint should be used for control.
// The priority is Orientation -> Heading -> No heading control.
func (sb *sensorBase) determineHeadingFunc(ctx context.Context,
	orientation, compassHeading movementsensor.MovementSensor,
) {
	switch {
	case orientation != nil:

		sb.logger.CInfof(ctx, "using sensor %s as angular heading sensor for base %v", orientation.Name().ShortName(), sb.Name().ShortName())

		sb.headingFunc = func(ctx context.Context) (float64, bool, error) {
			orient, err := orientation.Orientation(ctx, nil)
			if err != nil {
				return 0, false, err
			}
			// this returns (-180-> 180)
			yaw := rdkutils.RadToDeg(orient.EulerAngles().Yaw)

			return yaw, true, nil
		}
	case compassHeading != nil:
		sb.logger.CInfof(ctx, "using sensor %s as angular heading sensor for base %v", compassHeading.Name().ShortName(), sb.Name().ShortName())

		sb.headingFunc = func(ctx context.Context) (float64, bool, error) {
			compass, err := compassHeading.CompassHeading(ctx, nil)
			if err != nil {
				return 0, false, err
			}
			// flip compass heading to be CCW/Z up
			compass = 360 - compass

			// make the compass heading (-180->180)
			if compass > 180 {
				compass -= 360
			}

			return compass, true, nil
		}
	default:
		sb.logger.CInfof(ctx, "base %v cannot control heading, no heading related sensor given",
			sb.Name().ShortName())
		sb.headingFunc = func(ctx context.Context) (float64, bool, error) {
			return 0, false, nil
		}
	}
}

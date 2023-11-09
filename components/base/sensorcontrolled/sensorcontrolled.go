// Package sensorcontrolled base implements a base with feedback control from a movement sensor
package sensorcontrolled

import (
	"context"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

const (
	yawPollTime        = 5 * time.Millisecond
	velocitiesPollTime = 5 * time.Millisecond
	boundCheckTurn     = 2.0
	boundCheckTarget   = 5.0
	sensorDebug        = false
)

var (
	// Model is the name of the sensor_controlled model of a base component.
	Model           = resource.DefaultModelFamily.WithModel("sensor-controlled")
	errNoGoodSensor = errors.New("no appropriate sensor for orientation or velocity feedback")
)

// Config configures a sensor controlled base.
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
	logger logging.Logger
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

	loop *control.Loop
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

	if sb.velocities != nil {
		if err := setupControlLoops(sb); err != nil {
			return err
		}
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

	return sb.controlledBase.MoveStraight(ctx, distanceMm, mmPerSec, extra)
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

	if sb.loop != nil {
		sb.loop.Stop()
	}

	sb.activeBackgroundWorkers.Wait()
	return nil
}

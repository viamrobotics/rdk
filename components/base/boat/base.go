// Package boat implements a base for a boat with support for N motors in any position or angle
// This is an Experimental package
package boat

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	boatComp := registry.Component{
		Constructor: func(
			ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger,
		) (interface{}, error) {
			return createBoat(deps, config.ConvertedAttributes.(*boatConfig), logger)
		},
	}
	registry.RegisterComponent(base.Subtype, "boat", boatComp)

	config.RegisterComponentAttributeMapConverter(
		base.SubtypeName,
		"boat",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf boatConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&boatConfig{})
}

func createBoat(deps registry.Dependencies, config *boatConfig, logger golog.Logger) (base.LocalBase, error) {
	if config.WidthMM <= 0 {
		return nil, errors.New("width has to be > 0")
	}

	if config.LengthMM <= 0 {
		return nil, errors.New("length has to be > 0")
	}

	theBoat := &boat{cfg: config, logger: logger}

	for _, mc := range config.Motors {
		m, err := motor.FromDependencies(deps, mc.Name)
		if err != nil {
			return nil, err
		}
		theBoat.motors = append(theBoat.motors, m)
	}

	if config.IMU != "" {
		var err error
		theBoat.imu, err = movementsensor.FromDependencies(deps, config.IMU)
		if err != nil {
			return nil, err
		}
	}
	return theBoat, nil
}

type boatState struct {
	threadStarted      bool
	velocityControlled bool

	lastPower                               []float64
	lastPowerLinear, lastPowerAngular       r3.Vector
	velocityLinearGoal, velocityAngularGoal r3.Vector
}

type boat struct {
	generic.Unimplemented

	cfg    *boatConfig
	motors []motor.Motor
	imu    movementsensor.MovementSensor

	opMgr operation.SingleOperationManager

	state      boatState
	stateMutex sync.Mutex

	cancel    context.CancelFunc
	waitGroup sync.WaitGroup

	logger golog.Logger
}

func (b *boat) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	if distanceMm < 0 {
		mmPerSec *= -1
		distanceMm *= -1
	}
	err := b.SetVelocity(ctx, r3.Vector{Y: mmPerSec}, r3.Vector{}, extra)
	if err != nil {
		return err
	}
	s := time.Duration(float64(time.Millisecond) * math.Abs(float64(distanceMm)))
	utils.SelectContextOrWait(ctx, s)
	return b.Stop(ctx, nil)
}

func (b *boat) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	millis := 1000 * (angleDeg / degsPerSec)
	err := b.SetVelocity(ctx, r3.Vector{}, r3.Vector{Z: -1 * degsPerSec}, extra)
	if err != nil {
		return err
	}
	utils.SelectContextOrWait(ctx, time.Duration(float64(time.Millisecond)*millis))
	return b.Stop(ctx, nil)
}

func (b *boat) startVelocityThread() error {
	if b.imu == nil {
		return errors.New("no imu")
	}

	var ctx context.Context
	ctx, b.cancel = context.WithCancel(context.Background())

	b.waitGroup.Add(1)
	go func() {
		defer b.waitGroup.Done()

		for {
			utils.SelectContextOrWait(ctx, time.Millisecond*500)
			err := b.velocityThreadLoop(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				b.logger.Warn(err)
			}
		}
	}()

	return nil
}

func (b *boat) velocityThreadLoop(ctx context.Context) error {
	av, err := b.imu.AngularVelocity(ctx)
	if err != nil {
		return err
	}

	b.stateMutex.Lock()

	if !b.state.velocityControlled {
		b.stateMutex.Unlock()
		return nil
	}

	linear, angular := computeNextPower(&b.state, av, b.logger)

	b.stateMutex.Unlock()
	return b.setPowerInternal(ctx, linear, angular)
}

func computeNextPower(state *boatState, angularVelocity spatialmath.AngularVelocity, logger golog.Logger) (r3.Vector, r3.Vector) {
	linear := state.lastPowerLinear
	angular := state.lastPowerAngular

	angularDiff := angularVelocity.Z - state.velocityAngularGoal.Z

	if math.Abs(angularDiff) > 1 {
		delta := angularDiff / 360
		for math.Abs(delta) < .01 {
			delta *= 2
		}

		angular.Z -= delta * 10
		angular.Z = math.Max(-1, angular.Z)
		angular.Z = math.Min(1, angular.Z)
	}

	linear.Y = state.velocityLinearGoal.Y // TEMP
	linear.X = state.velocityLinearGoal.X // TEMP

	if logger != nil && true {
		logger.Debugf(
			"computeNextPower last: %0.2f %0.2f %0.2f goal v: %0.2f %0.2f %0.2f av: %0.2f"+
				" -> %0.2f %0.2f %0.2f",
			state.lastPowerLinear.X,
			state.lastPowerLinear.Y,
			state.lastPowerAngular.Z,
			state.velocityLinearGoal.X,
			state.velocityLinearGoal.Y,
			state.velocityAngularGoal.Z,
			angularVelocity.Z,
			linear.X, linear.Y, angular.Z,
		)
	}

	return linear, angular
}

func (b *boat) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	b.logger.Debugf("SetVelocity %v %v", linear, angular)
	_, done := b.opMgr.New(ctx)
	defer done()

	b.stateMutex.Lock()

	if !b.state.threadStarted {
		err := b.startVelocityThread()
		if err != nil {
			return err
		}
		b.state.threadStarted = true
	}

	b.state.velocityControlled = true
	b.state.velocityLinearGoal = linear
	b.state.velocityAngularGoal = angular
	b.stateMutex.Unlock()

	return nil
}

func (b *boat) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	b.logger.Debugf("SetPower %v %v", linear, angular)
	ctx, done := b.opMgr.New(ctx)
	defer done()

	b.stateMutex.Lock()
	b.state.velocityControlled = false
	b.stateMutex.Unlock()

	return b.setPowerInternal(ctx, linear, angular)
}

func (b *boat) setPowerInternal(ctx context.Context, linear, angular r3.Vector) error {
	power, err := b.cfg.computePower(linear, angular)
	if err != nil {
		return err
	}

	for idx, p := range power {
		err := b.motors[idx].SetPower(ctx, p, nil)
		if err != nil {
			return multierr.Combine(b.Stop(ctx, nil), err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	b.stateMutex.Lock()
	b.state.lastPower = power
	b.state.lastPowerLinear = linear
	b.state.lastPowerAngular = angular
	b.stateMutex.Unlock()

	return nil
}

func (b *boat) Stop(ctx context.Context, extra map[string]interface{}) error {
	b.stateMutex.Lock()
	b.state.velocityLinearGoal = r3.Vector{}
	b.state.velocityAngularGoal = r3.Vector{}
	b.stateMutex.Unlock()

	b.opMgr.CancelRunning(ctx)
	var err error
	for _, m := range b.motors {
		err = multierr.Combine(m.Stop(ctx, nil), err)
	}
	return err
}

func (b *boat) Width(ctx context.Context) (int, error) {
	return int(b.cfg.WidthMM), nil
}

func (b *boat) IsMoving(ctx context.Context) (bool, error) {
	for _, m := range b.motors {
		isMoving, err := m.IsPowered(ctx, nil)
		if err != nil {
			return false, err
		}
		if isMoving {
			return true, err
		}
	}
	return false, nil
}

func (b *boat) Close(ctx context.Context) error {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
		b.waitGroup.Wait()
	}
	return b.Stop(ctx, nil)
}

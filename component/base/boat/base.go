package boat

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
	
	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/utils"
)

func init() {
	boatComp := registry.Component{
		Constructor: func(
			ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger,
		) (interface{}, error) {
			return createBoat(ctx, r, config.ConvertedAttributes.(*boatConfig), logger)
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

func createBoat(ctx context.Context, r robot.Robot, config *boatConfig, logger golog.Logger) (base.LocalBase, error) {
	if config.Width <= 0 {
		return nil, errors.New("width has to be > 0")
	}

	if config.Length <= 0 {
		return nil, errors.New("length has to be > 0")
	}

	theBoat := &boat{cfg: config, logger: logger}

	for _, mc := range config.Motors {
		m, err := motor.FromRobot(r, mc.Name)
		if err != nil {
			return nil, err
		}
		theBoat.motors = append(theBoat.motors, m)
	}

	fmt.Printf("hi %#v\n", theBoat)

	if config.IMU != "" {
		var err error
		theBoat.imu, err = imu.FromRobot(r, config.IMU)
		if err != nil {
			return nil, err
		}
	}
	return theBoat, nil
}

type boatState struct {
	threadStarted bool
	velocityControlled bool

	lastPower []float64
	lastPowerLinear, lastPowerAngular r3.Vector
	velocityLinearGoal, velocityAngularGoal r3.Vector
}

type boat struct {
	generic.Unimplemented

	cfg    *boatConfig
	motors []motor.Motor
	imu imu.IMU
	
	opMgr operation.SingleOperationManager

	state boatState
	stateMutex sync.Mutex

	cancel context.CancelFunc

	logger golog.Logger
}

func (b *boat) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	panic(1)
}

func (b *boat) MoveArc(ctx context.Context, distanceMm int, mmPerSec float64, angleDeg float64) error {
	panic(1)
}

func (b *boat) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	panic(1)
}

func (b *boat) startVelocityThread() error {
	if b.imu == nil {
		return errors.New("no imu")
	}

	ctx := context.Background()
	ctx, b.cancel = context.WithCancel(ctx)
	
	go func() {
		for {
			utils.SelectContextOrWait(ctx, time.Millisecond * 100)
			err := b.velocityThreadLoop(ctx)
			if err != nil {
				b.logger.Warn(err)
			}
		}
	}()

	return nil
}

func (b *boat) velocityThreadLoop(ctx context.Context) error {

	av, err := b.imu.ReadAngularVelocity(ctx)
	if err != nil {
		return err
	}

	b.stateMutex.Lock()
	
	if !b.state.velocityControlled {
		b.stateMutex.Unlock()
		return nil
	}

	linear := b.state.lastPowerLinear
	angular := b.state.lastPowerAngular

	angularDiff := av.Z - b.state.velocityAngularGoal.Z
	
	if math.Abs(angularDiff) > 1 {
		delta := angularDiff / 360
		for math.Abs(delta) < .01 {
			delta *= 2
		}
		angular.Z -= delta
		angular.Z = math.Max(-1, angular.Z)
		angular.Z = math.Min(1, angular.Z)
	}

	fmt.Printf("prev: %v now: %v goal: %v diff: %v\n",
		b.state.lastPowerAngular.Z,
		angular.Z,
		b.state.velocityAngularGoal.Z,
		angularDiff,
	)
	b.stateMutex.Unlock()
	
	return b.setPowerInternal(ctx, linear, angular)
}

func (b *boat) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	ctx, done := b.opMgr.New(ctx)
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

	return b.setPowerInternal(ctx, linear, angular)
}


func (b *boat) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	ctx, done := b.opMgr.New(ctx)
	defer done()

	b.stateMutex.Lock()
	b.state.velocityControlled = false
	b.stateMutex.Unlock()

	return b.setPowerInternal(ctx, linear, angular)
}


func (b *boat) setPowerInternal(ctx context.Context, linear, angular r3.Vector) error {
	
	power := b.cfg.computePower(linear, angular)
	
	for idx, p := range power {
		err := b.motors[idx].SetPower(ctx, p)
		if err != nil {
			return multierr.Combine(b.Stop(ctx), err)
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

func (b *boat) Stop(ctx context.Context) error {
	b.opMgr.CancelRunning(ctx)
	var err error
	for _, m := range b.motors {
		err = multierr.Combine(m.Stop(ctx), err)
	}
	return err
}

func (b *boat) GetWidth(ctx context.Context) (int, error) {
	return int(b.cfg.Width) * 1000, nil
}

func (b *boat) Close() {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
}

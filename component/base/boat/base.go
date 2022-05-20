package boat

import (
	"context"
	"errors"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
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

	theBoat := &boat{cfg: config}

	for _, mc := range config.Motors {
		m, err := motor.FromRobot(r, mc.Name)
		if err != nil {
			return nil, err
		}
		theBoat.motors = append(theBoat.motors, m)
	}

	fmt.Printf("hi %#v\n", theBoat)

	return theBoat, nil
}

type boat struct {
	generic.Unimplemented

	cfg    *boatConfig
	motors []motor.Motor

	opMgr operation.SingleOperationManager
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

func (b *boat) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	power := b.cfg.computePower(linear, angular)

	ctx, done := b.opMgr.New(ctx)
	defer done()

	for idx, p := range power {
		err := b.motors[idx].SetPower(ctx, p)
		if err != nil {
			return multierr.Combine(b.Stop(ctx), err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

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

// Package mybase implements a base that "shimmies" forward one "foot" at a time.
package mybase

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mybase"),
)

func init() {
	registry.RegisterComponent(base.Subtype, Model, registry.Component{Constructor: newBase})
}

func newBase(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
	b := &MyBase{logger: logger}
	for n, d := range deps {
		switch n.Name {
		case config.Attributes.String("motorL"):
			m, ok := d.(motor.Motor)
			if !ok {
				return nil, errors.Errorf("resource %s is not a motor", n.Name)
			}
			b.left = m
		case config.Attributes.String("motorR"):
			m, ok := d.(motor.Motor)
			if !ok {
				return nil, errors.Errorf("resource %s is not a motor", n.Name)
			}
			b.right = m
		default:
			continue
		}
	}

	if b.left == nil || b.right == nil {
		return nil, errors.New("motorL and motorR must both be in depends_on")
	}

	return b, nil
}

type MyBase struct {
	generic.Unimplemented
	mu      sync.Mutex
	left    motor.Motor
	right   motor.Motor
	opMgr   operation.SingleOperationManager
	logger  golog.Logger
}

func (base *MyBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	base.logger.Debug("SMURF AHEAD")
	return base.SetPower(ctx, r3.Vector{X: mmPerSec}, r3.Vector{}, extra)
}

func (base *MyBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	base.logger.Debug("SMURF SPIN")
	var err error
	if angleDeg < 0 {
		multierr.Combine(err, base.left.SetPower(ctx, -1, extra))
		multierr.Combine(err, base.right.SetPower(ctx, 1, extra))		
	}else{
		multierr.Combine(err, base.left.SetPower(ctx, 1, extra))
		multierr.Combine(err, base.right.SetPower(ctx,-1, extra))		
	}

	if err != nil {
		return multierr.Combine(err, base.Stop(ctx, nil))
	}
	return nil
}

func (base *MyBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	base.logger.Debug("SMURF VELOCITY")
	return base.SetPower(ctx, linear, angular, extra)
}

func (base *MyBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	move, _ := base.left.IsPowered(ctx, nil)
	base.logger.Debug("SMURF POWER PRE %v", move)
	base.opMgr.CancelRunning(ctx)

	if linear.X < 0.01 { return base.Stop(ctx, nil)}

	// Send motor commands
	var err error
	multierr.Combine(err, base.left.SetPower(ctx, linear.X, extra))
	multierr.Combine(err, base.right.SetPower(ctx, linear.X, extra))

	time.Sleep(time.Second)
	move, _ = base.left.IsPowered(ctx, nil)
	base.logger.Debug("SMURF POWER POST %v", move)

	if err != nil {
		return multierr.Combine(err, base.Stop(ctx, nil))
	}

	return nil
}

func (base *MyBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	move, _ := base.left.IsPowered(ctx, nil)
	base.logger.Debug("SMURF STOP PRE %v", move)
	var err error
	for _, m := range []motor.Motor{base.left, base.right} {
		err = multierr.Combine(err, m.Stop(ctx, extra))
	}
	move, _ = base.left.IsPowered(ctx, nil)
	base.logger.Debug("SMURF STOP POST %v", move)
	return err
}

func (base *MyBase) IsMoving(ctx context.Context) (bool, error) {
	base.logger.Debug("SMURF ISMOVING")
	for _, m := range []motor.Motor{base.left, base.right} {
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

func (base *MyBase) Close(ctx context.Context) error {
	base.logger.Debug("SMURF CLOSE")
	return base.Stop(ctx, nil)
}

func (base *MyBase) Width(ctx context.Context) (int, error) {
	base.logger.Debug("SMURF WIDTH")
	return 42, nil
}


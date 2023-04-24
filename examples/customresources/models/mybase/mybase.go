// Package mybase implements a base that only supports SetPower (basic forward/back/turn controls.)
package mybase

import (
	"context"
	"fmt"
	"math"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
)

var (
	Model            = resource.NewModel("acme", "demo", "mybase")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(base.API, Model, resource.Registration[base.Base, *MyBaseConfig]{
		Constructor: newBase,
	})
}

func newBase(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (base.Base, error) {
	b := &MyBase{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := b.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return b, nil
}

func (base *MyBase) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	base.left = nil
	base.right = nil
	baseConfig, err := resource.NativeConfig[*MyBaseConfig](conf)
	if err != nil {
		return err
	}

	base.left, err = motor.FromDependencies(deps, baseConfig.LeftMotor)
	if err != nil {
		return errors.Wrapf(err, "unable to get motor %v for mybase", baseConfig.LeftMotor)
	}

	base.right, err = motor.FromDependencies(deps, baseConfig.RightMotor)
	if err != nil {
		return errors.Wrapf(err, "unable to get motor %v for mybase", baseConfig.RightMotor)
	}

	// Good practice to stop motors, but also this effectively tests https://viam.atlassian.net/browse/RSDK-2496
	return multierr.Combine(base.left.Stop(context.Background(), nil), base.right.Stop(context.Background(), nil))
}

func (base *MyBase) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

type MyBaseConfig struct {
	LeftMotor  string `json:"motorL"`
	RightMotor string `json:"motorR"`
}

func (cfg *MyBaseConfig) Validate(path string) ([]string, error) {
	if cfg.LeftMotor == "" {
		return nil, fmt.Errorf(`expected "motorL" attribute for mybase %q`, path)
	}
	if cfg.RightMotor == "" {
		return nil, fmt.Errorf(`expected "motorR" attribute for mybase %q`, path)
	}

	return []string{cfg.LeftMotor, cfg.RightMotor}, nil
}

type MyBase struct {
	resource.Named
	left   motor.Motor
	right  motor.Motor
	logger golog.Logger
}

func (base *MyBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	return errUnimplemented
}

func (base *MyBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	return errUnimplemented
}

func (base *MyBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return errUnimplemented
}

func (base *MyBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	base.logger.Debugf("SetPower Linear: %.2f Angular: %.2f", linear.Y, angular.Z)
	if math.Abs(linear.Y) < 0.01 && math.Abs(angular.Z) < 0.01 {
		return base.Stop(ctx, extra)
	}
	sum := math.Abs(linear.Y) + math.Abs(angular.Z)
	err1 := base.left.SetPower(ctx, (linear.Y-angular.Z)/sum, extra)
	err2 := base.right.SetPower(ctx, (linear.Y+angular.Z)/sum, extra)
	return multierr.Combine(err1, err2)
}

func (base *MyBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	base.logger.Debug("Stop")
	err1 := base.left.Stop(ctx, extra)
	err2 := base.right.Stop(ctx, extra)
	return multierr.Combine(err1, err2)
}

func (base *MyBase) IsMoving(ctx context.Context) (bool, error) {
	for _, m := range []motor.Motor{base.left, base.right} {
		isMoving, _, err := m.IsPowered(ctx, nil)
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
	return base.Stop(ctx, nil)
}

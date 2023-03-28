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
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var (
	Model            = resource.NewModel("acme", "demo", "mybase")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	registry.RegisterComponent(base.Subtype, Model, registry.Component{Constructor: newBase})

	// Use RegisterComponentAttributeMapConverter to register a custom configuration
	// struct that has a Validate(string) ([]string, error) method.
	//
	// The Validate method will automatically be called in RDK's module manager to
	// Validate the MyBase's configuration and register implicit dependencies.
	config.RegisterComponentAttributeMapConverter(
		base.Subtype,
		Model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf MyBaseConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&MyBaseConfig{})
}

func newBase(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
	b := &MyBase{logger: logger}
	err := b.Reconfigure(config, deps)
	return b, err
}

func (base *MyBase) Reconfigure(cfg config.Component, deps registry.Dependencies) error {
	base.left = nil
	base.right = nil
	for n, d := range deps {
		switch n.Name {
		case cfg.Attributes.String("motorL"):
			m, ok := d.(motor.Motor)
			if !ok {
				return errors.Errorf("resource %s is not a motor", n.Name)
			}
			base.left = m
		case cfg.Attributes.String("motorR"):
			m, ok := d.(motor.Motor)
			if !ok {
				return errors.Errorf("resource %s is not a motor", n.Name)
			}
			base.right = m
		default:
			continue
		}
	}

	if base.left == nil || base.right == nil {
		return errors.Errorf(`mybase %q needs both "motorL" and "motorR"`, cfg.Name)
	}
	return nil
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
	generic.Echo
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

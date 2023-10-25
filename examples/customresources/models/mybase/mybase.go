// Package mybase implements a base that only supports SetPower (basic forward/back/turn controls.)
package mybase

import (
	"context"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var (
	// Model is the full model definition.
	Model            = resource.NewModel("acme", "demo", "mybase")
	errUnimplemented = errors.New("unimplemented")
)

const (
	myBaseWidthMm        = 500.0 // our dummy base has a wheel tread of 500 millimeters
	myBaseTurningRadiusM = 0.3   // our dummy base turns around a circle of radius .3 meters
)

func init() {
	resource.RegisterComponent(base.API, Model, resource.Registration[base.Base, *Config]{
		Constructor: newBase,
	})
}

func newBase(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (base.Base, error) {
	b := &myBase{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := b.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return b, nil
}

// Reconfigure reconfigures with new settings.
func (b *myBase) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	b.left = nil
	b.right = nil

	// This takes the generic resource.Config passed down from the parent and converts it to the
	// model-specific (aka "native") Config structure defined above making it easier to directly access attributes.
	baseConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	b.left, err = motor.FromDependencies(deps, baseConfig.LeftMotor)
	if err != nil {
		return errors.Wrapf(err, "unable to get motor %v for mybase", baseConfig.LeftMotor)
	}

	b.right, err = motor.FromDependencies(deps, baseConfig.RightMotor)
	if err != nil {
		return errors.Wrapf(err, "unable to get motor %v for mybase", baseConfig.RightMotor)
	}

	geometries, err := kinematicbase.CollisionGeometry(conf.Frame)
	if err != nil {
		b.logger.Warnf("base %v %s", b.Name(), err.Error())
	}
	b.geometries = geometries

	// Good practice to stop motors, but also this effectively tests https://viam.atlassian.net/browse/RSDK-2496
	return multierr.Combine(b.left.Stop(context.Background(), nil), b.right.Stop(context.Background(), nil))
}

// DoCommand simply echos whatever was sent.
func (b *myBase) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

// Config contains two component (motor) names.
type Config struct {
	LeftMotor  string `json:"motorL"`
	RightMotor string `json:"motorR"`
}

// Validate validates the config and returns implicit dependencies,
// this Validate checks if the left and right motors exist for the module's base model.
func (cfg *Config) Validate(path string) ([]string, error) {
	// check if the attribute fields for the right and left motors are non-empty
	// this makes them reuqired for the model to successfully build
	if cfg.LeftMotor == "" {
		return nil, fmt.Errorf(`expected "motorL" attribute for mybase %q`, path)
	}
	if cfg.RightMotor == "" {
		return nil, fmt.Errorf(`expected "motorR" attribute for mybase %q`, path)
	}

	// Return the left and right motor names so that `newBase` can access them as dependencies.
	return []string{cfg.LeftMotor, cfg.RightMotor}, nil
}

type myBase struct {
	resource.Named
	left       motor.Motor
	right      motor.Motor
	logger     logging.Logger
	geometries []spatialmath.Geometry
}

// MoveStraight does nothing.
func (b *myBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
	return errUnimplemented
}

// Spin does nothing.
func (b *myBase) Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
	return errUnimplemented
}

// SetVelocity does nothing.
func (b *myBase) SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	return errUnimplemented
}

// SetPower computes relative power between the wheels and sets power for both motors.
func (b *myBase) SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
	b.logger.Debugf("SetPower Linear: %.2f Angular: %.2f", linear.Y, angular.Z)
	if math.Abs(linear.Y) < 0.01 && math.Abs(angular.Z) < 0.01 {
		return b.Stop(ctx, extra)
	}
	sum := math.Abs(linear.Y) + math.Abs(angular.Z)
	err1 := b.left.SetPower(ctx, (linear.Y-angular.Z)/sum, extra)
	err2 := b.right.SetPower(ctx, (linear.Y+angular.Z)/sum, extra)
	return multierr.Combine(err1, err2)
}

// Stop halts motion.
func (b *myBase) Stop(ctx context.Context, extra map[string]interface{}) error {
	b.logger.Debug("Stop")
	err1 := b.left.Stop(ctx, extra)
	err2 := b.right.Stop(ctx, extra)
	return multierr.Combine(err1, err2)
}

// IsMoving returns true if either motor is active.
func (b *myBase) IsMoving(ctx context.Context) (bool, error) {
	for _, m := range []motor.Motor{b.left, b.right} {
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

// Properties returns details about the physics of the base.
func (b *myBase) Properties(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
	return base.Properties{
		TurningRadiusMeters: myBaseTurningRadiusM,
		WidthMeters:         myBaseWidthMm * 0.001, // converting millimeters to meters
	}, nil
}

// Geometries returns physical dimensions.
func (b *myBase) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return b.geometries, nil
}

// Close stops motion during shutdown.
func (b *myBase) Close(ctx context.Context) error {
	return b.Stop(ctx, nil)
}

// Package fake implements a fake servo.
package fake

import (
	"context"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func init() {
	resource.RegisterComponent(
		servo.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[servo.Servo, resource.NoNativeConfig]{
			Constructor: func(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger) (servo.Servo, error) {
				return &Servo{
					Named:  conf.ResourceName().AsNamed(),
					logger: logger,
				}, nil
			},
		})
}

// A Servo allows setting and reading a single angle.
type Servo struct {
	angle uint32
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
}

// Move sets the given angle.
func (s *Servo) Move(ctx context.Context, angleDeg uint32, extra map[string]interface{}) error {
	s.angle = angleDeg
	return nil
}

// Position returns the set angle.
func (s *Servo) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	return s.angle, nil
}

// Stop doesn't do anything for a fake servo.
func (s *Servo) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving is always false for a fake servo.
func (s *Servo) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

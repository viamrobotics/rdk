// Package fake implements a fake servo.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(servo.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			var s servo.LocalServo = &Servo{Name: config.Name}
			return s, nil
		},
	})
}

// A Servo allows setting and reading a single angle.
type Servo struct {
	Name  string
	angle uint8
	generic.Echo
}

// Move sets the given angle.
func (s *Servo) Move(ctx context.Context, angleDeg uint8) error {
	s.angle = angleDeg
	return nil
}

// GetPosition returns the set angle.
func (s *Servo) GetPosition(ctx context.Context) (uint8, error) {
	return s.angle, nil
}

// Stop doesn't do anything for a fake servo.
func (s *Servo) Stop(ctx context.Context) error {
	return nil
}

// IsMoving is always false for a fake servo.
func (s *Servo) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

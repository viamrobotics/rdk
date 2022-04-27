// Package fake implements a fake servo.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(servo.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return &Servo{Name: config.Name}, nil
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

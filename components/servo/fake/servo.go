// Package fake implements a fake servo.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

func init() {
	registry.RegisterComponent(servo.Subtype, resource.NewDefaultModel("fake"), registry.Component{
		Constructor: func(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (resource.Resource, error) {
			return &Servo{
				Named: conf.ResourceName().AsNamed(),
			}, nil
		},
	})
}

// A Servo allows setting and reading a single angle.
type Servo struct {
	angle uint32
	resource.Named
	resource.TriviallyReconfigurable
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

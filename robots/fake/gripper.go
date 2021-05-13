package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

func init() {
	registry.RegisterGripper(ModelName, func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
		return &Gripper{Name: config.Name}, nil
	})
}

// Gripper is a fake gripper that can simply read and set properties.
type Gripper struct {
	Name string
}

// Open does nothing.
func (g *Gripper) Open(ctx context.Context) error {
	return nil
}

// Close does nothing.
func (g *Gripper) Close() error {
	return nil
}

// Grab does nothing.
func (g *Gripper) Grab(ctx context.Context) (bool, error) {
	return false, nil
}

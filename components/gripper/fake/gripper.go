// Package fake implements a fake gripper.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

var modelname = resource.NewDefaultModel("fake")

// Config is the config for a trossen gripper.
type Config struct{}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) error {
	return nil
}

func init() {
	resource.RegisterComponent(gripper.Subtype, modelname, resource.Registration[gripper.Gripper, *Config]{
		Constructor: func(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (gripper.Gripper, error) {
			return &Gripper{
				Named: conf.ResourceName().AsNamed(),
			}, nil
		},
		AttributeMapConverter: resource.TransformAttributeMap[*Config],
	})
}

// Gripper is a fake gripper that can simply read and set properties.
type Gripper struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

// ModelFrame returns the dynamic frame of the model.
func (g *Gripper) ModelFrame() referenceframe.Model {
	return nil
}

// Open does nothing.
func (g *Gripper) Open(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// Grab does nothing.
func (g *Gripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return false, nil
}

// Stop doesn't do anything for a fake gripper.
func (g *Gripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving is always false for a fake gripper.
func (g *Gripper) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

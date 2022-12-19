// Package fake implements a fake gripper.
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var modelname = resource.NewDefaultModel("fake")

// AttrConfig is the config for a trossen gripper.
type AttrConfig struct{}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	return nil
}

func init() {
	registry.RegisterComponent(gripper.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			var g gripper.LocalGripper = &Gripper{Name: config.Name}

			return g, nil
		},
	})

	config.RegisterComponentAttributeMapConverter(gripper.Subtype, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

// Gripper is a fake gripper that can simply read and set properties.
type Gripper struct {
	generic.Echo
	Name string
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

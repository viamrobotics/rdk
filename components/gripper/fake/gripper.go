// Package fake implements a fake gripper.
package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var model = resource.DefaultModelFamily.WithModel("fake")

// Config is the config for a trossen gripper.
type Config struct {
	resource.TriviallyValidateConfig
}

func init() {
	resource.RegisterComponent(gripper.API, model, resource.Registration[gripper.Gripper, *Config]{Constructor: NewGripper})
}

// Gripper is a fake gripper that can simply read and set properties.
type Gripper struct {
	resource.Named
	resource.TriviallyCloseable
	geometries []spatialmath.Geometry
	mu         sync.Mutex
	logger     golog.Logger
}

// NewGripper instantiates a new gripper of the fake model type.
func NewGripper(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (gripper.Gripper, error) {
	g := &Gripper{
		Named:      conf.ResourceName().AsNamed(),
		geometries: []spatialmath.Geometry{},
		logger:     logger,
	}
	if err := g.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return g, nil
}

// Reconfigure reconfigures the gripper atomically and in place.
func (g *Gripper) Reconfigure(_ context.Context, _ resource.Dependencies, conf resource.Config) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if conf.Frame != nil && conf.Frame.Geometry != nil {
		geometry, err := conf.Frame.Geometry.ParseConfig()
		if err != nil {
			return err
		}
		g.geometries = []spatialmath.Geometry{geometry}
	}
	return nil
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

// Geometries returns the geometries associated with the fake base.
func (g *Gripper) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.geometries, nil
}

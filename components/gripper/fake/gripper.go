// Package fake implements a fake gripper.
package fake

import (
	"context"
	"errors"
	"sync"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/logging"
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
	model      referenceframe.Model
	mu         sync.Mutex
	logger     logging.Logger
}

// NewGripper instantiates a new gripper of the fake model type.
func NewGripper(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (gripper.Gripper, error) {
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
	model, err := gripper.MakeModel(g.Name().ShortName(), g.geometries)
	if err != nil {
		return err
	}
	g.model = model
	return nil
}

// Kinematics returns the kinematic model associated with the gripper.
func (g *Gripper) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.model, nil
}

// CurrentInputs is unimplemented for grippers.
func (g *Gripper) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.model != nil && len(g.model.DoF()) != 0 {
		return nil, errors.New("CurrentInputs is unimplemented for gripper models with DoF != 0")
	}
	return []referenceframe.Input{}, nil
}

// GoToInputs is unimplemented for grippers.
func (g *Gripper) GoToInputs(context.Context, ...[]referenceframe.Input) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.model != nil && len(g.model.DoF()) != 0 {
		return errors.New("GoToInputs is unimplemented for gripper models with DoF != 0")
	}
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

// IsHoldingSomething always returns a status in which the gripper is not holding something and
// no additional information is supplied.
func (g *Gripper) IsHoldingSomething(ctx context.Context, extra map[string]interface{}) (gripper.HoldingStatus, error) {
	return gripper.HoldingStatus{}, nil
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

// Package fake implements a fake gripper.
package fake

import (
	"context"
	"sync"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var model = resource.DefaultModelFamily.WithModel("fake")

// Config is used for converting config attributes.
type Config struct {
	ModelFilePath string `json:"model-path,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	if conf.ModelFilePath != "" {
		if _, err := referenceframe.KinematicModelFromFile(conf.ModelFilePath, ""); err != nil {
			return nil, nil, err
		}
	}
	return nil, nil, nil
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
	inputs     []referenceframe.Input
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

	newConf := &Config{}
	if conf.ConvertedAttributes != nil {
		var err error
		newConf, err = resource.NativeConfig[*Config](conf)
		if err != nil {
			return err
		}
	}

	if conf.Frame != nil && conf.Frame.Geometry != nil {
		geometry, err := conf.Frame.Geometry.ParseConfig()
		if err != nil {
			return err
		}
		g.geometries = []spatialmath.Geometry{geometry}
	}

	if newConf.ModelFilePath != "" {
		model, err := referenceframe.KinematicModelFromFile(newConf.ModelFilePath, g.Name().ShortName())
		if err != nil {
			return err
		}
		g.model = model
	} else {
		model, err := gripper.MakeModel(g.Name().ShortName(), g.geometries)
		if err != nil {
			return err
		}
		g.model = model
	}
	g.inputs = make([]referenceframe.Input, len(g.model.DoF()))
	return nil
}

// Kinematics returns the kinematic model associated with the gripper.
func (g *Gripper) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.model, nil
}

// CurrentInputs returns the current inputs of the fake gripper.
func (g *Gripper) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ret := make([]referenceframe.Input, len(g.inputs))
	copy(ret, g.inputs)
	return ret, nil
}

// GoToInputs moves the fake gripper to the given inputs.
func (g *Gripper) GoToInputs(_ context.Context, inputSteps ...[]referenceframe.Input) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, inputs := range inputSteps {
		copy(g.inputs, inputs)
	}
	return nil
}

// Open sets all inputs to their maximum values (fully open).
func (g *Gripper) Open(ctx context.Context, extra map[string]interface{}) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	dof := g.model.DoF()
	for i, limit := range dof {
		g.inputs[i] = limit.Max
	}
	return nil
}

// Grab sets all inputs to their minimum values (fully closed).
func (g *Gripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	dof := g.model.DoF()
	for i, limit := range dof {
		g.inputs[i] = limit.Min
	}
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

// Geometries returns the geometries associated with the fake gripper at its current input positions.
func (g *Gripper) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.model.DoF()) > 0 {
		gif, err := g.model.Geometries(g.inputs)
		if err != nil {
			return nil, err
		}
		return gif.Geometries(), nil
	}
	return g.geometries, nil
}

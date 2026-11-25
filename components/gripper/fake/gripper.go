// Package fake implements a fake gripper.
package fake

import (
	"context"
	"fmt"
	"math"
	"sync"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model is the fake gripper model.
var Model = resource.DefaultModelFamily.WithModel("fake")

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
	resource.RegisterComponent(gripper.API, Model, resource.Registration[gripper.Gripper, *Config]{Constructor: NewGripper})
}

// Gripper is a fake gripper that can simply read and set properties.
type Gripper struct {
	resource.Named
	resource.TriviallyCloseable
	geometries []spatialmath.Geometry
	model      referenceframe.Model
	inputs     []referenceframe.Input
	// percentOpen is the opening from 0.0 (closed) to 1.0 (open).
	percentOpen float64
	mu          sync.Mutex
	logger      logging.Logger
}

// NewGripper instantiates a new gripper of the fake model type.
func NewGripper(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (gripper.Gripper, error) {
	g := &Gripper{
		Named:      conf.ResourceName().AsNamed(),
		geometries: []spatialmath.Geometry{},
		logger:     logger,
	}
	if err := g.reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return g, nil
}

// reconfigure reconfigures the gripper atomically and in place.
func (g *Gripper) reconfigure(_ context.Context, _ resource.Dependencies, conf resource.Config) error {
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
	g.percentOpen = g.percentOpenFromInputs()
	return nil
}

// percentOpenFromInputs derives the opening from the first input's position within its limits.
// A gripper with no DoF keeps its last opening. Must be called with mu held.
func (g *Gripper) percentOpenFromInputs() float64 {
	dof := g.model.DoF()
	if len(dof) == 0 || dof[0].Max == dof[0].Min {
		return g.percentOpen
	}
	p := (float64(g.inputs[0]) - dof[0].Min) / (dof[0].Max - dof[0].Min)
	return math.Max(0, math.Min(1, p))
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
	g.percentOpen = g.percentOpenFromInputs()
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
	g.percentOpen = 1
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
	g.percentOpen = 0
	return false, nil
}

// SetPercentOpen moves every input to the same fraction of its range, from 0.0 (closed, at the
// minimum) to 1.0 (open, at the maximum).
func (g *Gripper) SetPercentOpen(ctx context.Context, percentOpen float64, extra map[string]interface{}) error {
	if math.IsNaN(percentOpen) || percentOpen < 0 || percentOpen > 1 {
		return fmt.Errorf("percent_open must be between 0.0 and 1.0, got %v", percentOpen)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for i, limit := range g.model.DoF() {
		g.inputs[i] = limit.Min + percentOpen*(limit.Max-limit.Min)
	}
	g.percentOpen = percentOpen
	return nil
}

// PercentOpen returns the opening of the fake gripper, from 0.0 (closed) to 1.0 (open).
func (g *Gripper) PercentOpen(ctx context.Context, extra map[string]interface{}) (float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.percentOpen, nil
}

// Properties reports that the fake gripper supports SetPercentOpen and PercentOpen.
func (g *Gripper) Properties(ctx context.Context, extra map[string]interface{}) (gripper.Properties, error) {
	return gripper.Properties{PercentOpenSupported: true}, nil
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

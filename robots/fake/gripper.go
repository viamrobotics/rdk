package fake

import (
	"context"
	_ "embed" // for embedding model file

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

//go:embed gripper_model.json
var gripperjson []byte

func init() {
	registry.RegisterGripper(ModelName, registry.Gripper{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
			model, err := kinematics.ParseJSON(gripperjson, "")
			if err != nil {
				return nil, err
			}

			return &Gripper{Name: config.Name, model: model}, nil
		},
	})
}

// Gripper is a fake gripper that can simply read and set properties.
type Gripper struct {
	Name  string
	model *kinematics.Model
}

// ModelFrame returns the dynamic frame of the model
func (g *Gripper) ModelFrame() *kinematics.Model {
	return g.model
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

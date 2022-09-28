// Package trossen implements a trossen gripper.
package trossen

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "trossen", registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewGripper(deps, logger)
		},
	})
}

type Gripper struct {
	generic.Unimplemented
	a arm.LocalArm
}


func (g * Gripper) Grab(ctx context.Context) (bool, error) {
	resp, err := g.a.DoCommand(ctx, map[string]interface{}{"command": "grab"})
	if err != nil {
		return false, err
	}
	return resp["grabbed"].(bool), nil
}

func (g * Gripper) Open(ctx context.Context) error {
	_, err := g.a.DoCommand(ctx, map[string]interface{}{"command": "open"})
	return err
}

// Stop is unimplemented for Gripper.
func (g *Gripper) Stop(ctx context.Context) error {
	return g.a.Stop(ctx, nil)
}

// IsMoving returns whether the gripper is moving.
func (g *Gripper) IsMoving(ctx context.Context) (bool, error) {
	return g.a.IsMoving(ctx)
}

// ModelFrame is unimplemented for Gripper.
func (g *Gripper) ModelFrame() referenceframe.Model {
	return nil
}

func NewGripper(deps registry.Dependencies, logger golog.Logger) (gripper.LocalGripper, error) {
	if len(deps) != 1 {
		return nil, errors.New("gripper must have only one depends_on entry")
	}
	for _, d := range deps {
		a, ok := d.(arm.LocalArm)
		if ok {
			return &Gripper {a: a}, nil
		}
		return nil, errors.Errorf("need a trossen arm in depends_on, but saw %T", d)
	}
	return nil, errors.New("unpossible")
}

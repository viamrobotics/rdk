// Package yahboom implements a yahboom based gripper.
package yahboom

import (
	"context"
	// for embedding model file.
	_ "embed"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/arm/yahboom"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(gripper.Subtype, "yahboom-dofbot", registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGripper(deps, config)
		},
	})
}

func newGripper(deps registry.Dependencies, config config.Component) (gripper.LocalGripper, error) {
	armName := config.Attributes.String("arm")
	if armName == "" {
		return nil, errors.New("yahboom-dofbot gripper needs an arm")
	}
	myArm, err := arm.FromDependencies(deps, armName)
	if err != nil {
		return nil, err
	}

	dofArm, ok := utils.UnwrapProxy(myArm).(*yahboom.Dofbot)
	if !ok {
		return nil, fmt.Errorf("yahboom-dofbot gripper got not a dofbot arm, got %T", myArm)
	}
	g := &dofGripper{dofArm: dofArm}

	return g, nil
}

type dofGripper struct {
	generic.Unimplemented
	dofArm *yahboom.Dofbot

	opMgr operation.SingleOperationManager
}

func (g *dofGripper) Open(ctx context.Context) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.dofArm.Open(ctx)
}

func (g *dofGripper) Grab(ctx context.Context) (bool, error) {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.dofArm.Grab(ctx)
}

func (g *dofGripper) Stop(ctx context.Context) error {
	// RSDK-388: Implement Stop for gripper
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.dofArm.GripperStop(ctx)
}

// IsMoving returns whether the gripper is moving.
func (g *dofGripper) IsMoving() bool {
	return g.opMgr.OpRunning()
}

func (g *dofGripper) ModelFrame() referenceframe.Model {
	return g.dofArm.ModelFrame()
}

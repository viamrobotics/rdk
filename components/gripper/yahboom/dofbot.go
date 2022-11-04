// Package yahboom implements a yahboom based gripper.
package yahboom

import (
	"context"
	// for embedding model file.
	_ "embed"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	gutils "go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/yahboom"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
)

const modelname = "yahboom-dofbot"

// AttrConfig is the config for a dofbot gripper.
type AttrConfig struct {
	Arm string `json:"arm"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if config.Arm == "" {
		return nil, gutils.NewConfigValidationFieldRequiredError(path, "arm")
	}
	deps = append(deps, config.Arm)
	return deps, nil
}

func init() {
	registry.RegisterComponent(gripper.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGripper(deps, config)
		},
	})

	config.RegisterComponentAttributeMapConverter(gripper.SubtypeName, modelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

func newGripper(deps registry.Dependencies, config config.Component) (gripper.LocalGripper, error) {
	attr, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(attr, config.ConvertedAttributes)
	}
	armName := attr.Arm
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

func (g *dofGripper) Open(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.dofArm.Open(ctx)
}

func (g *dofGripper) Grab(ctx context.Context, extra map[string]interface{}) (bool, error) {
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.dofArm.Grab(ctx)
}

func (g *dofGripper) Stop(ctx context.Context, extra map[string]interface{}) error {
	// RSDK-388: Implement Stop for gripper
	ctx, done := g.opMgr.New(ctx)
	defer done()
	return g.dofArm.GripperStop(ctx)
}

// IsMoving returns whether the gripper is moving.
func (g *dofGripper) IsMoving(ctx context.Context) (bool, error) {
	return g.opMgr.OpRunning(), nil
}

func (g *dofGripper) ModelFrame() referenceframe.Model {
	return g.dofArm.ModelFrame()
}

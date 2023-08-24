// Package yahboom implements a yahboom based gripper.
// code with commands found at http://www.yahboom.net/study/Dofbot-Pi
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
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var model = resource.DefaultModelFamily.WithModel("yahboom-dofbot")

// Config is the config for a dofbot gripper.
type Config struct {
	Arm string `json:"arm"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string
	if conf.Arm == "" {
		return nil, gutils.NewConfigValidationFieldRequiredError(path, "arm")
	}
	deps = append(deps, conf.Arm)
	return deps, nil
}

func init() {
	resource.RegisterComponent(gripper.API, model, resource.Registration[gripper.Gripper, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (gripper.Gripper, error) {
			return newGripper(deps, conf, logger)
		},
	})
}

// newGripper instantiates a new Gripper of dofGripper type.
func newGripper(deps resource.Dependencies, conf resource.Config, logger golog.Logger) (gripper.Gripper, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	armName := newConf.Arm
	if armName == "" {
		return nil, errors.New("yahboom-dofbot gripper needs an arm")
	}
	myArm, err := arm.FromDependencies(deps, armName)
	if err != nil {
		return nil, err
	}

	dofArm, ok := myArm.(*yahboom.Dofbot)
	if !ok {
		return nil, fmt.Errorf("yahboom-dofbot gripper got not a dofbot arm, got %T", myArm)
	}
	g := &dofGripper{
		Named:      conf.ResourceName().AsNamed(),
		dofArm:     dofArm,
		opMgr:      operation.NewSingleOperationManager(),
		geometries: []spatialmath.Geometry{},
		logger:     logger,
	}

	if conf.Frame != nil && conf.Frame.Geometry != nil {
		geometry, err := conf.Frame.Geometry.ParseConfig()
		if err != nil {
			return nil, err
		}
		g.geometries = []spatialmath.Geometry{geometry}
	}

	return g, nil
}

type dofGripper struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	dofArm     *yahboom.Dofbot
	opMgr      *operation.SingleOperationManager
	geometries []spatialmath.Geometry
	logger     golog.Logger
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

// Geometries returns the geometries associated with the dofGripper.
func (g *dofGripper) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	return g.geometries, nil
}

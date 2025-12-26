// Package wrapper is a package that defines an implementation that wraps a partially implemented arm
package wrapper

import (
	"context"
	"sync"

	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Config is used for converting config attributes.
type Config struct {
	ModelFilePath string `json:"model-path"`
	ArmName       string `json:"arm-name"`
}

var model = resource.DefaultModelFamily.WithModel("wrapper_arm")

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, []string, error) {
	var deps []string
	if cfg.ArmName == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "arm-name")
	}
	if _, err := referenceframe.KinematicModelFromFile(cfg.ModelFilePath, ""); err != nil {
		return nil, nil, err
	}
	deps = append(deps, cfg.ArmName)
	return deps, nil, nil
}

func init() {
	resource.RegisterComponent(arm.API, model, resource.Registration[arm.Arm, *Config]{
		Constructor: NewWrapperArm,
	})
}

// Arm wraps a partial implementation of another arm.
type Arm struct {
	resource.Named
	resource.TriviallyCloseable
	logger logging.Logger
	opMgr  *operation.SingleOperationManager

	mu     sync.RWMutex
	model  referenceframe.Model
	actual arm.Arm
}

// NewWrapperArm returns a wrapper component for another arm.
func NewWrapperArm(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (arm.Arm, error) {
	a := &Arm{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		opMgr:  operation.NewSingleOperationManager(),
	}
	if err := a.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return a, nil
}

// Reconfigure atomically reconfigures this arm in place based on the new config.
func (wrapper *Arm) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	model, err := referenceframe.KinematicModelFromFile(newConf.ModelFilePath, conf.Name)
	if err != nil {
		return err
	}

	newArm, err := arm.FromProvider(deps, newConf.ArmName)
	if err != nil {
		return err
	}

	wrapper.mu.Lock()
	wrapper.model = model
	wrapper.actual = newArm
	wrapper.mu.Unlock()

	return nil
}

// EndPosition returns the set position.
func (wrapper *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()

	joints, err := wrapper.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.ComputeOOBPosition(wrapper.model, joints)
}

// MoveToPosition sets the position.
func (wrapper *Arm) MoveToPosition(ctx context.Context, pos spatialmath.Pose, extra map[string]interface{}) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()
	return armplanning.MoveArm(ctx, wrapper.logger, wrapper, pos)
}

// MoveToJointPositions sets the joints.
func (wrapper *Arm) MoveToJointPositions(ctx context.Context, joints []referenceframe.Input, extra map[string]interface{}) error {
	// check that joint positions are not out of bounds
	if err := arm.CheckDesiredJointPositions(ctx, wrapper, joints); err != nil {
		return err
	}
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()

	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()
	return wrapper.actual.MoveToJointPositions(ctx, joints, extra)
}

// MoveThroughJointPositions moves the arm sequentially through the given joints.
func (wrapper *Arm) MoveThroughJointPositions(
	ctx context.Context,
	positions [][]referenceframe.Input,
	_ *arm.MoveOptions,
	_ map[string]interface{},
) error {
	for _, goal := range positions {
		// check that joint positions are not out of bounds
		if err := arm.CheckDesiredJointPositions(ctx, wrapper, goal); err != nil {
			return err
		}
		err := wrapper.MoveToJointPositions(ctx, goal, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// JointPositions returns the set joints.
func (wrapper *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()

	joints, err := wrapper.actual.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return joints, nil
}

func (wrapper *Arm) StreamJointPositions(ctx context.Context, fps int32, extra map[string]interface{}) (chan *arm.JointPositionsStreamed, error) {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()
	return wrapper.actual.StreamJointPositions(ctx, fps, extra)
}

// Stop stops the actual arm.
func (wrapper *Arm) Stop(ctx context.Context, extra map[string]interface{}) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()

	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()
	return wrapper.actual.Stop(ctx, extra)
}

// IsMoving returns whether the arm is moving.
func (wrapper *Arm) IsMoving(ctx context.Context) (bool, error) {
	return wrapper.opMgr.OpRunning(), nil
}

// Kinematics returns the kinematic wrapper supplied to the wrapper arm.
func (wrapper *Arm) Kinematics(ctx context.Context) (referenceframe.Model, error) {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()
	return wrapper.model, nil
}

// CurrentInputs returns the current inputs of the arm.
func (wrapper *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return wrapper.actual.JointPositions(ctx, nil)
}

// GoToInputs moves the arm to the specified goal inputs.
func (wrapper *Arm) GoToInputs(ctx context.Context, inputSteps ...[]referenceframe.Input) error {
	return wrapper.MoveThroughJointPositions(ctx, inputSteps, nil, nil)
}

// Geometries returns the list of geometries associated with the resource, in any order. The poses of the geometries reflect their
// current location relative to the frame of the resource.
func (wrapper *Arm) Geometries(ctx context.Context, extra map[string]interface{}) ([]spatialmath.Geometry, error) {
	inputs, err := wrapper.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	gif, err := wrapper.model.Geometries(inputs)
	if err != nil {
		return nil, err
	}
	return gif.Geometries(), nil
}

// Get3DModels returns the 3D models of the arm.
func (wrapper *Arm) Get3DModels(ctx context.Context, extra map[string]interface{}) (map[string]*commonpb.Mesh, error) {
	models, err := wrapper.actual.Get3DModels(ctx, extra)
	if err != nil {
		return nil, err
	}
	return models, nil
}

// modelFromPath returns a Model from a given path.

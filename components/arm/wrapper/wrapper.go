// Package wrapper is a package that defines an implementation that wraps a partially implemented arm
package wrapper

import (
	"context"
	"sync"

	pb "go.viam.com/api/component/arm/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
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
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.ArmName == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "arm-name")
	}
	if _, err := referenceframe.ModelFromPath(cfg.ModelFilePath, ""); err != nil {
		return nil, err
	}
	deps = append(deps, cfg.ArmName)
	return deps, nil
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
	model, err := referenceframe.ModelFromPath(newConf.ModelFilePath, conf.Name)
	if err != nil {
		return err
	}

	newArm, err := arm.FromDependencies(deps, newConf.ArmName)
	if err != nil {
		return err
	}

	wrapper.mu.Lock()
	wrapper.model = model
	wrapper.actual = newArm
	wrapper.mu.Unlock()

	return nil
}

// ModelFrame returns the dynamic frame of the model.
func (wrapper *Arm) ModelFrame() referenceframe.Model {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()
	return wrapper.model
}

// EndPosition returns the set position.
func (wrapper *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()

	joints, err := wrapper.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputeOOBPosition(wrapper.model, joints)
}

// MoveToPosition sets the position.
func (wrapper *Arm) MoveToPosition(ctx context.Context, pos spatialmath.Pose, extra map[string]interface{}) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, wrapper.logger, wrapper, pos)
}

// MoveToJointPositions sets the joints.
func (wrapper *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
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

// JointPositions returns the set joints.
func (wrapper *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()

	joints, err := wrapper.actual.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return joints, nil
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

// CurrentInputs returns the current inputs of the arm.
func (wrapper *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	wrapper.mu.RLock()
	defer wrapper.mu.RUnlock()
	res, err := wrapper.actual.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return wrapper.model.InputFromProtobuf(res), nil
}

// GoToInputs moves the arm to the specified goal inputs.
func (wrapper *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	// check that joint positions are not out of bounds
	positionDegs := wrapper.model.ProtobufFromInput(goal)
	if err := arm.CheckDesiredJointPositions(ctx, wrapper, positionDegs); err != nil {
		return err
	}
	return wrapper.MoveToJointPositions(ctx, positionDegs, nil)
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

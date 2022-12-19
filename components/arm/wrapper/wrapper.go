// Package wrapper is a package that defines an implementation that wraps a partially implemented arm
package wrapper

import (
	"context"
	"errors"
	"strings"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	ModelPath string `json:"model-path"`
	ArmName   string `json:"arm-name"`
}

var model = resource.NewDefaultModel("wrapper_arm")

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.ArmName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "arm-name")
	}
	deps = append(deps, cfg.ArmName)
	return deps, nil
}

func init() {
	registry.RegisterComponent(arm.Subtype, model, registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewWrapperArm(config, r, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.Subtype, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)
}

// Arm wraps a partial implementation of another arm.
type Arm struct {
	generic.Unimplemented
	Name   string
	model  referenceframe.Model
	actual arm.Arm
	logger golog.Logger
	robot  robot.Robot
	opMgr  operation.SingleOperationManager
}

// NewWrapperArm returns a wrapper component for another arm.
func NewWrapperArm(cfg config.Component, r robot.Robot, logger golog.Logger) (arm.LocalArm, error) {
	var (
		model referenceframe.Model
		err   error
	)
	modelPath := cfg.ConvertedAttributes.(*AttrConfig).ModelPath
	switch {
	case strings.HasSuffix(modelPath, ".urdf"):
		model, err = referenceframe.ParseURDFFile(cfg.ConvertedAttributes.(*AttrConfig).ModelPath, cfg.Name)
	case strings.HasSuffix(modelPath, ".json"):
		model, err = referenceframe.ParseModelJSONFile(cfg.ConvertedAttributes.(*AttrConfig).ModelPath, cfg.Name)
	default:
		return nil, errors.New("unsupported kinematic model encoding file passed")
	}

	if err != nil {
		return nil, err
	}
	wrappedArm, err := arm.FromRobot(r, cfg.ConvertedAttributes.(*AttrConfig).ArmName)
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:   cfg.Name,
		model:  model,
		actual: wrappedArm,
		logger: logger,
		robot:  r,
	}, nil
}

// ModelFrame returns the dynamic frame of the model.
func (wrapper *Arm) ModelFrame() referenceframe.Model {
	return wrapper.model
}

// EndPosition returns the set position.
func (wrapper *Arm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	joints, err := wrapper.JointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(wrapper.model, joints)
}

// MoveToPosition sets the position.
func (wrapper *Arm) MoveToPosition(
	ctx context.Context,
	pos spatialmath.Pose,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()
	return arm.Move(ctx, wrapper.robot, wrapper, pos, worldState)
}

// MoveToJointPositions sets the joints.
func (wrapper *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()

	return wrapper.actual.MoveToJointPositions(ctx, joints, extra)
}

// JointPositions returns the set joints.
func (wrapper *Arm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
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

	return wrapper.actual.Stop(ctx, extra)
}

// IsMoving returns whether the arm is moving.
func (wrapper *Arm) IsMoving(ctx context.Context) (bool, error) {
	return wrapper.opMgr.OpRunning(), nil
}

// CurrentInputs returns the current inputs of the arm.
func (wrapper *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := wrapper.actual.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return wrapper.model.InputFromProtobuf(res), nil
}

// GoToInputs moves the arm to the specified goal inputs.
func (wrapper *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return wrapper.MoveToJointPositions(ctx, wrapper.model.ProtobufFromInput(goal), nil)
}

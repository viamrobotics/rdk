// Package wrapper is a package that defines an implementation that wraps a partially implemented arm
package wrapper

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	ModelPath string `json:"model-path"`
}

func init() {
	registry.RegisterComponent(arm.Subtype, "wrapper_arm", registry.Component{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			childArm, err := arm.FromRobot(r, config.Name)
			if err != nil {
				return nil, err
			}
			return NewWrapperArm(config, childArm, r, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.SubtypeName, "wrapper_arm",
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
func NewWrapperArm(cfg config.Component, actual arm.Arm, r robot.Robot, logger golog.Logger) (arm.LocalArm, error) {
	model, err := referenceframe.ParseModelJSONFile(cfg.ConvertedAttributes.(AttrConfig).ModelPath, cfg.Name)
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:   cfg.Name + "_wrapper",
		model:  model,
		actual: actual,
		logger: logger,
		robot:  r,
	}, nil
}

// ModelFrame returns the dynamic frame of the model.
func (wrapper *Arm) ModelFrame() referenceframe.Model {
	return wrapper.model
}

// GetEndPosition returns the set position.
func (wrapper *Arm) GetEndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	joints, err := wrapper.GetJointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(wrapper.model, joints)
}

// MoveToPosition sets the position.
func (wrapper *Arm) MoveToPosition(
	ctx context.Context,
	pos *commonpb.Pose,
	worldState *commonpb.WorldState,
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

// GetJointPositions returns the set joints.
func (wrapper *Arm) GetJointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	joints, err := wrapper.actual.GetJointPositions(ctx, extra)
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
	res, err := wrapper.actual.GetJointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return wrapper.model.InputFromProtobuf(res), nil
}

// GoToInputs moves the arm to the specified goal inputs.
func (wrapper *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return wrapper.MoveToJointPositions(ctx, wrapper.model.ProtobufFromInput(goal), nil)
}

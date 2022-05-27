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
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			childArm, err := arm.FromRobot(r, config.Name)
			if err != nil {
				return nil, err
			}
			return NewWrapperArm(config, childArm, logger)
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
	mp     motionplan.MotionPlanner
	opMgr  operation.SingleOperationManager
}

// NewWrapperArm returns a wrapper component for another arm.
func NewWrapperArm(cfg config.Component, actual arm.Arm, logger golog.Logger) (arm.Arm, error) {
	model, err := referenceframe.ParseModelJSONFile(cfg.ConvertedAttributes.(*AttrConfig).ModelPath, cfg.Name)
	if err != nil {
		return nil, err
	}
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, 4, logger)
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:   cfg.Name + "_wrapper",
		model:  model,
		actual: actual,
		logger: logger,
		mp:     mp,
	}, nil
}

// ModelFrame returns the dynamic frame of the model.
func (wrapper *Arm) ModelFrame() referenceframe.Model {
	return wrapper.model
}

// GetEndPosition returns the set position.
func (wrapper *Arm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := wrapper.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(wrapper.model, joints)
}

// MoveToPosition sets the position.
func (wrapper *Arm) MoveToPosition(ctx context.Context, pose *commonpb.Pose, worldState *commonpb.WorldState) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()

	joints, err := wrapper.actual.GetJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := wrapper.mp.Plan(ctx, pose, referenceframe.JointPosToInputs(joints), nil)
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, wrapper, solution)
}

// MoveToJointPositions sets the joints.
func (wrapper *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	ctx, done := wrapper.opMgr.New(ctx)
	defer done()

	return wrapper.actual.MoveToJointPositions(ctx, joints)
}

// GetJointPositions returns the set joints.
func (wrapper *Arm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	joints, err := wrapper.actual.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return joints, nil
}

// Stop stops the actual arm.
func (wrapper *Arm) Stop(ctx context.Context) error {
	wrapper.opMgr.CancelRunning(ctx)
	return wrapper.actual.Stop(ctx)
}

// CurrentInputs returns the current inputs of the arm.
func (wrapper *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := wrapper.actual.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

// GoToInputs moves the arm to the specified goal inputs.
func (wrapper *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return wrapper.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

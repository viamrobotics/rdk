// Package wrapper is a package that defines an implementation that wraps a partially implemented arm
package wrapper

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// AttrConfig is used for converting config attributes.
type AttrConfig struct {
	ModelPath string
}

func init() {
	registry.RegisterComponent(arm.Subtype, "wrapper", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			resource, err := r.ResourceByName(resource.NameFromSubtype(arm.Subtype, config.Name))
			if err != nil {
				return nil, err
			}
			if childArm, ok := resource.(arm.Arm); !ok {
				return NewWrapperArm(config, childArm, logger)
			}
			return nil, err
		},
	})

	config.RegisterComponentAttributeMapConverter(arm.SubtypeName, "wrapper",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)
}

// Arm is an arm that wraps a partial implementation of an arm.
type Arm struct {
	generic.Unimplemented
	Name   string
	model  referenceframe.Model
	actual arm.Arm
	logger golog.Logger
	mp     motionplan.MotionPlanner
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
func (a *Arm) ModelFrame() referenceframe.Model {
	return a.model
}

// GetEndPosition returns the set position.
func (a *Arm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := a.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(a.model, joints)
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, pose *commonpb.Pose, worldState *commonpb.WorldState) error {
	joints, err := a.GetJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := a.mp.Plan(ctx, pose, referenceframe.JointPosToInputs(joints), nil)
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, a, solution)
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	return a.actual.MoveToJointPositions(ctx, joints)
}

// GetJointPositions returns the set joints.
func (a *Arm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	joints, err := a.actual.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return joints, nil
}

// CurrentInputs returns the current inputs of the arm.
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

// GoToInputs moves the arm to the specified goal inputs.
func (a *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return a.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

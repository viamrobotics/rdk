// parentarm is a package that defines an implementation that wraps a partially implemented child arm
package parentarm

import (
	"context"

	// used to import model referenceframe.
	_ "embed"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

//go:embed hdt_model.json
var remoteModelJSON []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			resource, err := r.ResourceByName(resource.NameFromSubtype(arm.Subtype, config.Name))
			if err != nil {
				return nil, err
			}
			if childArm, ok := resource.(arm.Arm); !ok {
				return NewParentArm(config, childArm, logger)
			}
			return nil, err
		},
	})
}

// NewArm returns pa new remote arm.
func NewParentArm(cfg config.Component, child arm.Arm, logger golog.Logger) (arm.Arm, error) {
	model, err := referenceframe.UnmarshalModelJSON(remoteModelJSON, "")
	if err != nil {
		return nil, err
	}
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, 4, logger)
	if err != nil {
		return nil, err
	}
	return &ParentArm{
		Name:   cfg.Name + "_parent",
		model:  model,
		child:  child,
		logger: logger,
		mp:     mp,
	}, nil
}

// ParentArm is an arm that wraps a partial implementation of an arm
type ParentArm struct {
	Name   string
	model  referenceframe.Model
	child  arm.Arm
	logger golog.Logger
	mp     motionplan.MotionPlanner
}

// ModelFrame returns the dynamic frame of the model.
func (pa *ParentArm) ModelFrame() referenceframe.Model {
	return pa.model
}

// GetEndPosition returns the set position.
func (pa *ParentArm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	joints, err := pa.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(pa.model, joints)
}

// MoveToPosition sets the position.
func (pa *ParentArm) MoveToPosition(ctx context.Context, pose *commonpb.Pose, worldState *commonpb.WorldState) error {
	joints, err := pa.GetJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := pa.mp.Plan(ctx, pose, referenceframe.JointPosToInputs(joints), nil)
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, pa, solution)
}

// MoveToJointPositions sets the joints.
func (pa *ParentArm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	pa.child.MoveToJointPositions(ctx, joints)
	return nil
}

// GetJointPositions returns the set joints.
func (pa *ParentArm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	joints, err := pa.child.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return joints, nil
}

// CurrentInputs TODO.
func (pa *ParentArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := pa.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

// GoToInputs TODO.
func (pa *ParentArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return pa.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

// Close does nothing.
func (pa *ParentArm) Close() {
	return
}

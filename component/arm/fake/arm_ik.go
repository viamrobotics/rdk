package fake

import (
	"context"
	// for arm model.
	_ "embed"
	"math"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
)

//go:embed arm_model.json
var armikModelJSON []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "fake_ik", registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewArmIK(ctx, config, logger)
		},
	})
}

// NewArmIK returns a new fake arm.
func NewArmIK(ctx context.Context, cfg config.Component, logger golog.Logger) (arm.LocalArm, error) {
	name := cfg.Name
	model, err := referenceframe.UnmarshalModelJSON(armikModelJSON, "")
	if err != nil {
		return nil, err
	}
	mp, err := motionplan.NewCBiRRTMotionPlanner(model, 4, logger)
	if err != nil {
		return nil, err
	}
	return &ArmIK{
		Name:     name,
		position: &commonpb.Pose{},
		joints:   &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 0}},
		mp:       mp,
		model:    model,
	}, nil
}

// ArmIK is a fake arm that can simply read and set properties.
type ArmIK struct {
	generic.Echo
	Name       string
	position   *commonpb.Pose
	joints     *pb.JointPositions
	mp         motionplan.MotionPlanner
	CloseCount int
	model      referenceframe.Model
}

// ModelFrame returns the dynamic frame of the model.
func (a *ArmIK) ModelFrame() referenceframe.Model {
	return a.model
}

// GetEndPosition returns the set position.
func (a *ArmIK) GetEndPosition(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
	joints, err := a.GetJointPositions(ctx, extra)
	if err != nil {
		return nil, err
	}
	return motionplan.ComputePosition(a.mp.Frame(), joints)
}

// MoveToPosition sets the position.
func (a *ArmIK) MoveToPosition(
	ctx context.Context,
	pos *commonpb.Pose,
	worldState *commonpb.WorldState,
	extra map[string]interface{},
) error {
	joints, err := a.GetJointPositions(ctx, extra)
	if err != nil {
		return err
	}
	solution, err := a.mp.Plan(ctx, pos, a.model.InputFromProtobuf(joints), nil)
	if err != nil {
		return err
	}
	return arm.GoToWaypoints(ctx, a, solution)
}

// MoveToJointPositions sets the joints.
func (a *ArmIK) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions, extra map[string]interface{}) error {
	for _, val := range joints.Values {
		if math.Abs(val) > 360 {
			return errors.New("invalid joint location")
		}
	}

	a.joints.Values = joints.Values
	return nil
}

// GetJointPositions returns joints.
func (a *ArmIK) GetJointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	retJoint := pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 0}}
	retJoint.Values = a.joints.Values
	return &retJoint, nil
}

// Stop doesn't do anything for a fake arm.
func (a *ArmIK) Stop(ctx context.Context, extra map[string]interface{}) error {
	return nil
}

// IsMoving is always false for a fake arm.
func (a *ArmIK) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

// CurrentInputs TODO.
func (a *ArmIK) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.GetJointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return a.model.InputFromProtobuf(res), nil
}

// GoToInputs TODO.
func (a *ArmIK) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return a.MoveToJointPositions(ctx, a.model.ProtobufFromInput(goal), nil)
}

// Close does nothing.
func (a *ArmIK) Close() {
	a.CloseCount++
}

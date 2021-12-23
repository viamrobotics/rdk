package fake

import (
	"context"
	_ "embed" // used to import model frame

	"github.com/pkg/errors"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"

	"github.com/edaniels/golog"
)

//go:embed static_arm_model.json
var armModelJSON []byte

func init() {
	registry.RegisterComponent(arm.Subtype, ModelName, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewArm(config)
		},
	})
}

// NewArm returns a new fake arm.
func NewArm(cfg config.Component) (arm.Arm, error) {
	name := cfg.Name
	model, err := referenceframe.ParseJSON(armModelJSON, "")
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:     name,
		position: &commonpb.Pose{},
		joints:   &pb.ArmJointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
		model:    model,
	}, nil
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	Name       string
	position   *commonpb.Pose
	joints     *pb.ArmJointPositions
	CloseCount int
	model      *referenceframe.Model
}

// ModelFrame returns the dynamic frame of the model
func (a *Arm) ModelFrame() *referenceframe.Model {
	return a.model
}

// CurrentPosition returns the set position.
func (a *Arm) CurrentPosition(ctx context.Context) (*commonpb.Pose, error) {
	return a.position, nil
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, c *commonpb.Pose) error {
	a.position = c
	return nil
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.ArmJointPositions) error {
	a.joints = joints
	return nil
}

// CurrentJointPositions returns the set joints.
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
	return a.joints, nil
}

// JointMoveDelta returns an error.
func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("arm JointMoveDelta does nothing")
}

// CurrentInputs TODO
func (a *Arm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	res, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.JointPosToInputs(res), nil
}

// GoToInputs TODO
func (a *Arm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return a.MoveToJointPositions(ctx, referenceframe.InputsToJointPos(goal))
}

// Close does nothing.
func (a *Arm) Close() error {
	a.CloseCount++
	return nil
}

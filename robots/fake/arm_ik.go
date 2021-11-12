package fake

import (
	"context"
	_ "embed" // for arm model

	"github.com/go-errors/errors"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

//go:embed arm_model.json
var armikModelJSON []byte

func init() {
	registry.RegisterComponent(arm.Subtype, "fake_ik", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewArmIK(ctx, config, logger)
		},
	})
}

// fakeModel returns the kinematics model
func fakeModel() (*kinematics.Model, error) {
	return kinematics.ParseJSON(armikModelJSON, "")
}

// NewArmIK returns a new fake arm.
func NewArmIK(ctx context.Context, cfg config.Component, logger golog.Logger) (arm.Arm, error) {
	name := cfg.Name
	model, err := fakeModel()
	if err != nil {
		return nil, err
	}
	ik, err := kinematics.CreateCombinedIKSolver(model, logger, 4)
	if err != nil {
		return nil, err
	}

	return &ArmIK{
		Name:     name,
		position: &pb.Pose{},
		joints:   &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
		ik:       ik,
		model:    model,
	}, nil
}

// ArmIK is a fake arm that can simply read and set properties.
type ArmIK struct {
	Name       string
	position   *pb.Pose
	joints     *pb.JointPositions
	ik         kinematics.InverseKinematics
	CloseCount int
	model      *kinematics.Model
}

// ModelFrame returns the dynamic frame of the model
func (a *ArmIK) ModelFrame() *kinematics.Model {
	return a.model
}

// CurrentPosition returns the set position.
func (a *ArmIK) CurrentPosition(ctx context.Context) (*pb.Pose, error) {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return kinematics.ComputePosition(a.ik.Model(), joints)
}

// MoveToPosition sets the position.
func (a *ArmIK) MoveToPosition(ctx context.Context, pos *pb.Pose) error {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := a.ik.Solve(ctx, pos, frame.JointPosToInputs(joints))
	if err != nil {
		return err
	}
	return a.MoveToJointPositions(ctx, frame.InputsToJointPos(solution))
}

// MoveToJointPositions sets the joints.
func (a *ArmIK) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	a.joints = joints
	return nil
}

// CurrentJointPositions returns the set joints.
func (a *ArmIK) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return a.joints, nil
}

// JointMoveDelta returns an error.
func (a *ArmIK) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("arm JointMoveDelta does nothing")
}

// Close does nothing.
func (a *ArmIK) Close() error {
	a.CloseCount++
	return nil
}

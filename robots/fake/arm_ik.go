package fake

import (
	"context"
	_ "embed" // for arm model

	"github.com/go-errors/errors"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

//go:embed arm_model.json
var armModelJSON string

func init() {
	registry.RegisterArm("fake_ik", registry.Arm{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		if config.Attributes.Bool("fail_new", false) {
			return nil, errors.New("whoops")
		}
		return NewArmIK(config.Name, logger)
	}})
}

// NewArmIK returns a new fake arm.
func NewArmIK(name string, logger golog.Logger) (*ArmIK, error) {
	model, err := kinematics.ParseJSON([]byte(armModelJSON))
	if err != nil {
		return nil, err
	}

	ik := kinematics.CreateCombinedIKSolver(model, logger, 4)

	return &ArmIK{
		Name:     name,
		position: &pb.ArmPosition{},
		joints:   &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
		ik:       ik,
	}, nil
}

// ArmIK is a fake arm that can simply read and set properties.
type ArmIK struct {
	Name       string
	position   *pb.ArmPosition
	joints     *pb.JointPositions
	ik         kinematics.InverseKinematics
	CloseCount int
}

// CurrentPosition returns the set position.
func (a *ArmIK) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return nil, err
	}
	return kinematics.ComputePosition(a.ik.Model(), joints)
}

// MoveToPosition sets the position.
func (a *ArmIK) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
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

package fake

import (
	"context"
	_ "embed" // for arm model

	"github.com/go-errors/errors"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
)

//go:embed arm_model.json
var armModelJSON string

func init() {
	registry.RegisterArm("fake", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		if config.Attributes.Bool("fail_new", false) {
			return nil, errors.New("whoops")
		}
		return NewArm(config.Name, logger)
	})
}

// NewArm returns a new fake arm.
func NewArm(name string, logger golog.Logger) (*Arm, error) {
	model, err := kinematics.ParseJSON([]byte(armModelJSON))
	if err != nil {
		return nil, err
	}

	ik := kinematics.CreateCombinedIKSolver(model, logger, 4)

	return &Arm{
		Name:     name,
		position: &pb.ArmPosition{},
		joints:   &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
		ik:       ik,
	}, nil
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	Name       string
	position   *pb.ArmPosition
	joints     *pb.JointPositions
	ik         kinematics.InverseKinematics
	CloseCount int
}

// CurrentPosition returns the set position.
func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	joints, err := a.CurrentJointPositions(ctx)
	return kinematics.ComputePosition(a.ik.Mdl(), joints), err
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, pos *pb.ArmPosition) error {
	joints, err := a.CurrentJointPositions(ctx)
	if err != nil {
		return err
	}
	solution, err := a.ik.Solve(ctx, pos, joints)
	if err != nil {
		return err
	}
	return a.MoveToJointPositions(ctx, solution)
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	a.joints = joints
	return nil
}

// CurrentJointPositions returns the set joints.
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return a.joints, nil
}

// JointMoveDelta returns an error.
func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("arm JointMoveDelta does nothing")
}

// Close does nothing.
func (a *Arm) Close() error {
	a.CloseCount++
	return nil
}

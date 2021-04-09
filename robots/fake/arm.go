package fake

import (
	"context"
	"fmt"

	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
)

func init() {
	api.RegisterArm("fake", func(ctx context.Context, r api.Robot, config api.Component, logger golog.Logger) (api.Arm, error) {
		return NewArm(), nil
	})
}

func NewArm() *Arm {
	return &Arm{
		position: &pb.ArmPosition{},
		joints:   &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
	}
}

type Arm struct {
	position *pb.ArmPosition
	joints   *pb.JointPositions
}

func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	return a.position, nil
}

func (a *Arm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	a.position = c
	return nil
}

func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	a.joints = joints
	return nil
}

func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return a.joints, nil
}

func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amount float64) error {
	return fmt.Errorf("arm JointMoveDelta does nothing")
}

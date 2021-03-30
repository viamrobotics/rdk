package fake

import (
	"fmt"

	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
)

func init() {
	api.RegisterArm("fake", func(r api.Robot, config api.Component, logger golog.Logger) (api.Arm, error) {
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

func (a *Arm) Close() {
}

func (a *Arm) CurrentPosition() (*pb.ArmPosition, error) {
	return a.position, nil
}

func (a *Arm) MoveToPosition(c *pb.ArmPosition) error {
	a.position = c
	return nil
}

func (a *Arm) MoveToJointPositions(joints *pb.JointPositions) error {
	a.joints = joints
	return nil
}

func (a *Arm) CurrentJointPositions() (*pb.JointPositions, error) {
	return a.joints, nil
}

func (a *Arm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("arm JointMoveDelta does nothing")
}

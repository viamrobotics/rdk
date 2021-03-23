package fake

import (
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterArm("fake", func(r api.Robot, config api.Component, logger golog.Logger) (api.Arm, error) {
		return NewArm(), nil
	})
}

func NewArm() *Arm {
	return &Arm{
		position: api.ArmPosition{},
		joints:   api.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
	}
}

type Arm struct {
	position api.ArmPosition
	joints   api.JointPositions
}

func (a *Arm) Close() {
}

func (a *Arm) CurrentPosition() (api.ArmPosition, error) {
	return a.position, nil
}

func (a *Arm) MoveToPosition(c api.ArmPosition) error {
	a.position = c
	return nil
}

func (a *Arm) MoveToJointPositions(joints api.JointPositions) error {
	a.joints = joints
	return nil
}

func (a *Arm) CurrentJointPositions() (api.JointPositions, error) {
	return a.joints, nil
}

func (a *Arm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("arm JointMoveDelta does nothing")
}

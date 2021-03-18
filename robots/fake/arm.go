package fake

import (
	"fmt"

	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterArm("fake", func(r api.Robot, config api.Component) (api.Arm, error) {
		return &Arm{}, nil
	})
}

type Arm struct {
	position api.ArmPosition
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
	return fmt.Errorf("arm MoveToJointPositions doesn't work")
}

func (a *Arm) CurrentJointPositions() (api.JointPositions, error) {
	return api.JointPositions{}, fmt.Errorf("arm CurrentJointPositions doesn't work")
}

func (a *Arm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("arm JointMoveDelta does nothing")
}

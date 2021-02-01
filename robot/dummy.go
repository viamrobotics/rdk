package robot

import (
	"fmt"

	"github.com/echolabsinc/robotcore/arm"
)

type dummyGripper struct {
}

func (dg *dummyGripper) Open() error {
	return nil
}

func (dg *dummyGripper) Close() error {
	return nil
}

func (dg *dummyGripper) Grab() (bool, error) {
	return false, nil
}

// ----------

type dummyArm struct {
	position arm.Position
}

func (da *dummyArm) Close() {
}

func (da *dummyArm) CurrentPosition() (arm.Position, error) {
	return da.position, nil
}

func (da *dummyArm) MoveToPosition(c arm.Position) error {
	da.position = c
	return nil
}

func (da *dummyArm) MoveToJointPositions(joints []float64) error {
	return fmt.Errorf("dummyArm::MoveToJointPositions doesn't work")
}

func (da *dummyArm) CurrentJointPositions() ([]float64, error) {
	return nil, fmt.Errorf("dummyArm::CurrentJointPositions doesn't work")
}

func (da *dummyArm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("dummy JointMoveDelta does nothing")
}

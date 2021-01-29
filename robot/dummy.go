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
	state arm.RobotState
}

func (da *dummyArm) Close() {
}

func (da *dummyArm) State() arm.RobotState {
	return da.state
}

func (da *dummyArm) MoveToPositionC(c arm.CartesianInfo) error {
	da.state.CartesianInfo = c
	return nil
}

func (da *dummyArm) MoveToPosition(x, y, z, rx, ry, rz float64) error {
	da.state.CartesianInfo.X = x
	da.state.CartesianInfo.Y = z
	da.state.CartesianInfo.Z = z
	da.state.CartesianInfo.Rx = rx
	da.state.CartesianInfo.Ry = rx
	da.state.CartesianInfo.Rz = rz
	return nil
}

func (da *dummyArm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("dummy JointMoveDelta does nothing")
}

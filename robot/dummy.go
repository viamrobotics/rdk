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
	position arm.CartesianInfo
}

func (da *dummyArm) Close() {
}

func (da *dummyArm) CurrentPosition() (arm.CartesianInfo, error) {
	return da.position, nil
}

func (da *dummyArm) MoveToPositionC(c arm.CartesianInfo) error {
	da.position = c
	return nil
}

func (da *dummyArm) MoveToPosition(x, y, z, rx, ry, rz float64) error {
	da.position.X = x
	da.position.Y = z
	da.position.Z = z
	da.position.Rx = rx
	da.position.Ry = rx
	da.position.Rz = rz
	return nil
}

func (da *dummyArm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("dummy JointMoveDelta does nothing")
}

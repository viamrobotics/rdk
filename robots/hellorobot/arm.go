package hellorobot

import (
	"fmt"
	"math"
	"time"

	"github.com/viamrobotics/robotcore/arm"

	"github.com/sbinet/go-python"
)

type Arm struct {
	robot  *Robot
	armObj *python.PyObject
}

const armMoveSpeed = 1.0 / 4 // m/sec

func (a *Arm) MoveBy(meters float64) {
	a.armObj.CallMethod("move_by", python.PyFloat_FromDouble(meters))
	a.robot.pushCommand()
	time.Sleep(time.Duration(math.Ceil(math.Abs(meters)/armMoveSpeed)) * time.Second)
}

func (a *Arm) CurrentPosition() (arm.Position, error) {
	return arm.Position{}, fmt.Errorf("arm CurrentPosition doesn't work")
}

func (a *Arm) MoveToPosition(c arm.Position) error {
	return fmt.Errorf("arm MoveToPosition doesn't work")
}

func (a *Arm) MoveToJointPositions(joints arm.JointPositions) error {
	return fmt.Errorf("arm MoveToJointPositions doesn't work")
}

func (a *Arm) CurrentJointPositions() (arm.JointPositions, error) {
	return arm.JointPositions{}, fmt.Errorf("arm CurrentJointPositions doesn't work")
}

func (a *Arm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("arm JointMoveDelta does nothing")
}

func (a *Arm) Close() {
}

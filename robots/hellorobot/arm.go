package hellorobot

import (
	"fmt"
	"math"
	"time"

	"go.viam.com/robotcore/api"

	"github.com/sbinet/go-python"
)

func init() {
	api.RegisterArm(ModelName, func(r api.Robot, config api.Component) (api.Arm, error) {
		t := r.ProviderByModel(ModelName)
		if t == nil {
			return nil, fmt.Errorf("no provider created for %s", ModelName)
		}
		return t.(*Robot).Arm()
	})
}

type Arm struct {
	robot  *Robot
	armObj *python.PyObject
}

const armMoveSpeed = 1.0 / 4 // meters/sec

func (a *Arm) MoveBy(meters float64) error {
	a.armObj.CallMethod("move_by", python.PyFloat_FromDouble(meters))
	if err := checkPythonErr(); err != nil {
		return err
	}
	if err := a.robot.pushCommand(); err != nil {
		return err
	}
	time.Sleep(time.Duration(math.Ceil(math.Abs(meters)/armMoveSpeed)) * time.Second)
	return nil
}

func (a *Arm) CurrentPosition() (api.ArmPosition, error) {
	return api.ArmPosition{}, fmt.Errorf("arm CurrentPosition doesn't work")
}

func (a *Arm) MoveToPosition(c api.ArmPosition) error {
	return fmt.Errorf("arm MoveToPosition doesn't work")
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

func (a *Arm) Close() {
}

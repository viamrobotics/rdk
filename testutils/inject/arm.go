package inject

import (
	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/api/v1"
)

type Arm struct {
	api.Arm
	CurrentPositionFunc       func() (*pb.ArmPosition, error)
	MoveToPositionFunc        func(c *pb.ArmPosition) error
	MoveToJointPositionsFunc  func(*pb.JointPositions) error
	CurrentJointPositionsFunc func() (*pb.JointPositions, error)
	JointMoveDeltaFunc        func(joint int, amount float64) error
	CloseFunc                 func()
}

func (a *Arm) CurrentPosition() (*pb.ArmPosition, error) {
	if a.CurrentPositionFunc == nil {
		return a.Arm.CurrentPosition()
	}
	return a.CurrentPositionFunc()
}

func (a *Arm) MoveToPosition(c *pb.ArmPosition) error {
	if a.MoveToPositionFunc == nil {
		return a.Arm.MoveToPosition(c)
	}
	return a.MoveToPositionFunc(c)
}

func (a *Arm) MoveToJointPositions(jp *pb.JointPositions) error {
	if a.MoveToJointPositionsFunc == nil {
		return a.Arm.MoveToJointPositions(jp)
	}
	return a.MoveToJointPositionsFunc(jp)
}

func (a *Arm) CurrentJointPositions() (*pb.JointPositions, error) {
	if a.CurrentJointPositionsFunc == nil {
		return a.Arm.CurrentJointPositions()
	}
	return a.CurrentJointPositionsFunc()
}

func (a *Arm) JointMoveDelta(joint int, amount float64) error {
	if a.JointMoveDeltaFunc == nil {
		return a.Arm.JointMoveDelta(joint, amount)
	}
	return a.JointMoveDeltaFunc(joint, amount)
}

func (a *Arm) Close() {
	if a.CloseFunc == nil {
		a.Arm.Close()
		return
	}
	a.CloseFunc()
}

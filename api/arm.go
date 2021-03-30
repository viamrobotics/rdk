package api

import (
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"
)

func NewPositionFromMetersAndRadians(x, y, z, rx, ry, rz float64) *pb.ArmPosition {
	return &pb.ArmPosition{
		X:  x,
		Y:  y,
		Z:  z,
		RX: utils.RadToDeg(rx),
		RY: utils.RadToDeg(ry),
		RZ: utils.RadToDeg(rz),
	}
}

func JointPositionsToRadians(jp *pb.JointPositions) []float64 {
	n := make([]float64, len(jp.Degrees))
	for idx, d := range jp.Degrees {
		n[idx] = utils.DegToRad(d)
	}
	return n
}

func JointPositionsFromRadians(radians []float64) *pb.JointPositions {
	n := make([]float64, len(radians))
	for idx, a := range radians {
		n[idx] = utils.RadToDeg(a)
	}
	return &pb.JointPositions{Degrees: n}
}

// -----

type Arm interface {
	CurrentPosition() (*pb.ArmPosition, error)
	MoveToPosition(c *pb.ArmPosition) error

	MoveToJointPositions(*pb.JointPositions) error
	CurrentJointPositions() (*pb.JointPositions, error)

	JointMoveDelta(joint int, amount float64) error // TODO(erh): make it clear the units

	Close()
}

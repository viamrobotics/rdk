package api

import (
	"context"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"
)

func NewPositionFromMetersAndRadians(x, y, z, rx, ry, rz float64) *pb.ArmPosition {
	return &pb.ArmPosition{
		X:  int64(x * 1000),
		Y:  int64(y * 1000),
		Z:  int64(z * 1000),
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

// return millimeters away
func ArmPositionGridDiff(a, b *pb.ArmPosition) float64 {
	diff := utils.Square(float64(a.X-b.X)) +
		utils.Square(float64(a.Y-b.Y)) +
		utils.Square(float64(a.Z-b.Z))

	return utils.CubeRoot(diff)
}

// return degrees away?
func ArmPositionRotationDiff(a, b *pb.ArmPosition) float64 {
	diff := utils.Square(a.RX-b.RX) +
		utils.Square(a.RY-b.RY) +
		utils.Square(a.RZ-b.RZ)

	return utils.CubeRoot(diff)
}

// -----

type Arm interface {
	CurrentPosition(ctx context.Context) (*pb.ArmPosition, error)
	MoveToPosition(ctx context.Context, c *pb.ArmPosition) error

	MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error
	CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error)

	JointMoveDelta(ctx context.Context, joint int, amount float64) error // TODO(erh): make it clear the units
}

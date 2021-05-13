// Package arm defines the arm that a robot uses to manipulate objects.
package arm

import (
	"context"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"
)

// An Arm represents a physical robotic arm that exists in three-dimensional space.
type Arm interface {
	// CurrentPosition returns the current position of the arm.
	CurrentPosition(ctx context.Context) (*pb.ArmPosition, error)

	// MoveToPosition moves the arm to the given absolute position.
	MoveToPosition(ctx context.Context, c *pb.ArmPosition) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error

	// CurrentJointPositions returns the current joint positions of the arm.
	CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error)

	// JointMoveDelta moves a specific join of the arm by the given amount.
	JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error
}

// NewPositionFromMetersAndRadians returns a three-dimensional arm position
// defined by a point in space in meters and an orientation defined in radians.
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

// JointPositionsToRadians converts the given positions into a slice
// of radians.
func JointPositionsToRadians(jp *pb.JointPositions) []float64 {
	n := make([]float64, len(jp.Degrees))
	for idx, d := range jp.Degrees {
		n[idx] = utils.DegToRad(d)
	}
	return n
}

// JointPositionsFromRadians converts the given slice of radians into
// joint positions (represented in degrees).
func JointPositionsFromRadians(radians []float64) *pb.JointPositions {
	n := make([]float64, len(radians))
	for idx, a := range radians {
		n[idx] = utils.RadToDeg(a)
	}
	return &pb.JointPositions{Degrees: n}
}

// PositionGridDiff returns the euclidean distance between
// two arm positions in millimeters.
func PositionGridDiff(a, b *pb.ArmPosition) float64 {
	diff := utils.Square(float64(a.X-b.X)) +
		utils.Square(float64(a.Y-b.Y)) +
		utils.Square(float64(a.Z-b.Z))

	return utils.CubeRoot(diff)
}

// PositionRotationDiff returns the rotational distance
// between two arm positions in degrees.
func PositionRotationDiff(a, b *pb.ArmPosition) float64 {
	diff := utils.Square(a.RX-b.RX) +
		utils.Square(a.RY-b.RY) +
		utils.Square(a.RZ-b.RZ)

	return utils.CubeRoot(diff)
}

// Package arm defines the arm that a robot uses to manipulate objects.
package arm

import (
	"context"
	"math"

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

	// JointMoveDelta moves a specific joint of the arm by the given amount.
	JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error
}

// NewPositionFromMetersAndOV returns a three-dimensional arm position
// defined by a point in space in meters and an orientation defined as an OrientationVec.
// See robot.proto for a math explanation
func NewPositionFromMetersAndOV(x, y, z, th, ox, oy, oz float64) *pb.ArmPosition {
	return &pb.ArmPosition{
		X:     x * 1000,
		Y:     y * 1000,
		Z:     z * 1000,
		OX:    ox,
		OY:    oy,
		OZ:    oz,
		Theta: th,
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
	diff := utils.Square(a.X-b.X) +
		utils.Square(a.Y-b.Y) +
		utils.Square(a.Z-b.Z)

	// Pythagorean theorum in 3d uses sqrt, not cube root
	// https://www.mathsisfun.com/geometry/pythagoras-3d.html
	return math.Sqrt(diff)
}

// PositionRotationDiff returns the sum of the squared differences between the angle axis components of two positions
func PositionRotationDiff(a, b *pb.ArmPosition) float64 {
	return utils.Square(a.Theta-b.Theta) +
		utils.Square(a.OX-b.OX) +
		utils.Square(a.OY-b.OY) +
		utils.Square(a.OZ-b.OZ)
}

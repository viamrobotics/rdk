package arm

import (
	"math"
	"testing"

	"go.viam.com/core/kinematics/kinmath"
	pb "go.viam.com/core/proto/api/v1"

	"go.viam.com/test"
)

func TestArmPosition(t *testing.T) {
	p := NewPositionFromMetersAndOV(1.0, 2.0, 3.0, math.Pi/2, 0, 0.7071, 0.7071)

	test.That(t, p.OX, test.ShouldEqual, 0.0)
	test.That(t, p.OY, test.ShouldEqual, 0.7071)
	test.That(t, p.OZ, test.ShouldEqual, 0.7071)

	test.That(t, p.Theta, test.ShouldEqual, math.Pi/2)
}

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j.Degrees[0], test.ShouldEqual, 0.0)
	test.That(t, j.Degrees[1], test.ShouldEqual, 180.0)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}

func TestArmPositionDiff(t *testing.T) {
	test.That(t, PositionGridDiff(&pb.ArmPosition{}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 0)
	test.That(t, PositionGridDiff(&pb.ArmPosition{X: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionGridDiff(&pb.ArmPosition{Y: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionGridDiff(&pb.ArmPosition{Z: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionGridDiff(&pb.ArmPosition{X: 1, Y: 1, Z: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, math.Sqrt(3))

	test.That(t, PositionRotationDiff(&pb.ArmPosition{}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 0)
	test.That(t, PositionRotationDiff(&pb.ArmPosition{OX: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionRotationDiff(&pb.ArmPosition{OY: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionRotationDiff(&pb.ArmPosition{OZ: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionRotationDiff(&pb.ArmPosition{OX: 1, OY: 1, OZ: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 3)
}

func TestAAConversion(t *testing.T) {
	r3 := kinmath.R3AA{1.5, 1.5, 1.5}
	r4 := r3.ToR4()
	test.That(t, r4.Theta, test.ShouldAlmostEqual, 2.598076211353316)
	test.That(t, r4.RX, test.ShouldAlmostEqual, 0.5773502691896257)
	test.That(t, r4.RY, test.ShouldAlmostEqual, 0.5773502691896257)
	test.That(t, r4.RZ, test.ShouldAlmostEqual, 0.5773502691896257)
	r3_2 := r4.ToR3()
	test.That(t, r3_2.RX, test.ShouldAlmostEqual, 1.5)
	test.That(t, r3_2.RY, test.ShouldAlmostEqual, 1.5)
	test.That(t, r3_2.RZ, test.ShouldAlmostEqual, 1.5)

}

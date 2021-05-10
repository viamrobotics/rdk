package api

import (
	"math"
	"testing"

	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/utils"

	"go.viam.com/test"
)

func TestArmPosition(t *testing.T) {
	p := NewPositionFromMetersAndRadians(1.0, 2.0, 3.0, 0, math.Pi/2, math.Pi)

	test.That(t, p.RX, test.ShouldEqual, 0.0)
	test.That(t, p.RY, test.ShouldEqual, 90.0)
	test.That(t, p.RZ, test.ShouldEqual, 180.0)

	test.That(t, utils.DegToRad(p.RX), test.ShouldEqual, 0.0)
	test.That(t, utils.DegToRad(p.RY), test.ShouldEqual, math.Pi/2)
	test.That(t, utils.DegToRad(p.RZ), test.ShouldEqual, math.Pi)
}

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j.Degrees[0], test.ShouldEqual, 0.0)
	test.That(t, j.Degrees[1], test.ShouldEqual, 180.0)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}

func TestArmPositionDiff(t *testing.T) {
	test.That(t, ArmPositionGridDiff(&pb.ArmPosition{}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 0)
	test.That(t, ArmPositionGridDiff(&pb.ArmPosition{X: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, ArmPositionGridDiff(&pb.ArmPosition{Y: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, ArmPositionGridDiff(&pb.ArmPosition{Z: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, ArmPositionGridDiff(&pb.ArmPosition{X: 1, Y: 1, Z: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, utils.CubeRoot(3))

	test.That(t, ArmPositionRotationDiff(&pb.ArmPosition{}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 0)
	test.That(t, ArmPositionRotationDiff(&pb.ArmPosition{RX: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, ArmPositionRotationDiff(&pb.ArmPosition{RY: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, ArmPositionRotationDiff(&pb.ArmPosition{RZ: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, 1)
	test.That(t, ArmPositionRotationDiff(&pb.ArmPosition{RX: 1, RY: 1, RZ: 1}, &pb.ArmPosition{}), test.ShouldAlmostEqual, utils.CubeRoot(3))
}

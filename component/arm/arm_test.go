package arm

import (
	"context"
	"math"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"

	"go.viam.com/test"
)

func TestArmName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "8ad23fcd-7f30-56b9-a7f4-cf37a980b4dd",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"arm1",
			resource.Name{
				UUID: "1ef3fc81-df1d-5ac4-b11d-bc1513e47f06",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceCore, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "arm1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, tc.Expected, test.ShouldResemble, observed)
		})
	}
}

func TestWrapWtihReconfigurable(t *testing.T) {
	actualArm1 := &mockArm{Name: "arm1"}
	fakeArm1, err := WrapWithReconfigurable(actualArm1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeArm1.(*reconfigurableArm).actual, test.ShouldEqual, actualArm1)
}

func TestReconfigurableArm(t *testing.T) {
	actualArm1 := &mockArm{Name: "arm1"}
	fakeArm1, err := WrapWithReconfigurable(actualArm1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeArm1.(*reconfigurableArm).actual, test.ShouldEqual, actualArm1)

	actualArm2 := &mockArm{Name: "arm2"}
	fakeArm2, err := WrapWithReconfigurable(actualArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualArm1.reconCount, test.ShouldEqual, 0)

	err = fakeArm1.(*reconfigurableArm).Reconfigure(fakeArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeArm1.(*reconfigurableArm).actual, test.ShouldEqual, actualArm2)
	test.That(t, actualArm1.reconCount, test.ShouldEqual, 1)
}

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

type mockArm struct {
	Name       string
	reconCount int
}

func (m *mockArm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) { return nil, nil }

func (m *mockArm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error { return nil }

func (m *mockArm) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error { return nil }

func (m *mockArm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return nil, nil
}

func (m *mockArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return nil
}

func (m *mockArm) Close() error { m.reconCount++; return nil }

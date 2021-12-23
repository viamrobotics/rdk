package arm

import (
	"context"
	"math"
	"testing"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"

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
				UUID: "a5b161b9-dfa9-5eef-93d1-58431fd91212",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			"arm1",
			resource.Name{
				UUID: "ded8a90b-0c77-5bda-baf5-b7e79bbdb28a",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: SubtypeName,
				},
				Name: "arm1",
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualArm1 Arm = &mockArm{Name: "arm1"}
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
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 0)

	err = fakeArm1.(*reconfigurableArm).Reconfigure(fakeArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeArm1.(*reconfigurableArm).actual, test.ShouldEqual, actualArm2)
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 1)
}

func TestArmPosition(t *testing.T) {
	p := NewPositionFromMetersAndOV(1.0, 2.0, 3.0, math.Pi/2, 0, 0.7071, 0.7071)

	test.That(t, p.OX, test.ShouldEqual, 0.0)
	test.That(t, p.OY, test.ShouldEqual, 0.7071)
	test.That(t, p.OZ, test.ShouldEqual, 0.7071)

	test.That(t, p.Theta, test.ShouldEqual, math.Pi/2)
}

func TestArmPositionDiff(t *testing.T) {
	test.That(t, PositionGridDiff(&commonpb.Pose{}, &commonpb.Pose{}), test.ShouldAlmostEqual, 0)
	test.That(t, PositionGridDiff(&commonpb.Pose{X: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionGridDiff(&commonpb.Pose{Y: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionGridDiff(&commonpb.Pose{Z: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionGridDiff(&commonpb.Pose{X: 1, Y: 1, Z: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, math.Sqrt(3))

	test.That(t, PositionRotationDiff(&commonpb.Pose{}, &commonpb.Pose{}), test.ShouldAlmostEqual, 0)
	test.That(t, PositionRotationDiff(&commonpb.Pose{OX: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionRotationDiff(&commonpb.Pose{OY: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionRotationDiff(&commonpb.Pose{OZ: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, PositionRotationDiff(&commonpb.Pose{OX: 1, OY: 1, OZ: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 3)
}

type mockArm struct {
	Name        string
	reconfCount int
}

func (m *mockArm) CurrentPosition(ctx context.Context) (*commonpb.Pose, error) { return nil, nil }

func (m *mockArm) MoveToPosition(ctx context.Context, c *commonpb.Pose) error { return nil }

func (m *mockArm) MoveToJointPositions(ctx context.Context, pos *pb.ArmJointPositions) error {
	return nil
}

func (m *mockArm) CurrentJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
	return nil, nil
}

func (m *mockArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return nil
}

func (m *mockArm) ModelFrame() *referenceframe.Model {
	return nil
}

func (m *mockArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	return nil, nil
}

func (m *mockArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	return nil
}

func (m *mockArm) Close() error { m.reconfCount++; return nil }

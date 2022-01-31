package arm_test

import (
	"context"
	"math"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/sensor"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testArmName    = "arm1"
	testArmName2   = "arm2"
	failArmName    = "arm3"
	fakeArmName    = "arm4"
	missingArmName = "arm5"
)

func setupInjectRobot() *inject.Robot {
	arm1 := &mock{Name: testArmName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case arm.Named(testArmName):
			return arm1, true
		case arm.Named(fakeArmName):
			return "not a arm", false
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named(testArmName), sensor.Named("sensor1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	a, ok := arm.FromRobot(r, testArmName)
	test.That(t, a, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	pose1, err := a.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose1, test.ShouldResemble, pose)

	a, ok = arm.FromRobot(r, fakeArmName)
	test.That(t, a, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	a, ok = arm.FromRobot(r, missingArmName)
	test.That(t, a, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := arm.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testArmName})
}

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
					ResourceSubtype: arm.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testArmName,
			resource.Name{
				UUID: "ded8a90b-0c77-5bda-baf5-b7e79bbdb28a",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: arm.SubtypeName,
				},
				Name: testArmName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := arm.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualArm1 arm.Arm = &mock{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1)
	test.That(t, err, test.ShouldBeNil)

	_, err = arm.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

	reconfArm2, err := arm.WrapWithReconfigurable(reconfArm1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm2, test.ShouldEqual, reconfArm1)
}

func TestReconfigurableArm(t *testing.T) {
	actualArm1 := &mock{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1)
	test.That(t, err, test.ShouldBeNil)

	actualArm2 := &mock{Name: testArmName2}
	reconfArm2, err := arm.WrapWithReconfigurable(actualArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 0)

	err = reconfArm1.Reconfigure(context.Background(), reconfArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm1, test.ShouldResemble, reconfArm2)
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualArm1.endPosCount, test.ShouldEqual, 0)
	test.That(t, actualArm2.endPosCount, test.ShouldEqual, 0)
	pose1, err := reconfArm1.(arm.Arm).GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose1, test.ShouldResemble, pose)
	test.That(t, actualArm1.endPosCount, test.ShouldEqual, 0)
	test.That(t, actualArm2.endPosCount, test.ShouldEqual, 1)

	err = reconfArm1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *arm.reconfigurableArm")
}

func TestClose(t *testing.T) {
	actualArm1 := &mock{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfArm1), test.ShouldBeNil)
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 1)
}

func TestArmPosition(t *testing.T) {
	p := arm.NewPositionFromMetersAndOV(1.0, 2.0, 3.0, math.Pi/2, 0, 0.7071, 0.7071)

	test.That(t, p.OX, test.ShouldEqual, 0.0)
	test.That(t, p.OY, test.ShouldEqual, 0.7071)
	test.That(t, p.OZ, test.ShouldEqual, 0.7071)

	test.That(t, p.Theta, test.ShouldEqual, math.Pi/2)
}

func TestArmPositionDiff(t *testing.T) {
	test.That(t, arm.PositionGridDiff(&commonpb.Pose{}, &commonpb.Pose{}), test.ShouldAlmostEqual, 0)
	test.That(t, arm.PositionGridDiff(&commonpb.Pose{X: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, arm.PositionGridDiff(&commonpb.Pose{Y: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, arm.PositionGridDiff(&commonpb.Pose{Z: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, arm.PositionGridDiff(&commonpb.Pose{X: 1, Y: 1, Z: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, math.Sqrt(3))

	test.That(t, arm.PositionRotationDiff(&commonpb.Pose{}, &commonpb.Pose{}), test.ShouldAlmostEqual, 0)
	test.That(t, arm.PositionRotationDiff(&commonpb.Pose{OX: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, arm.PositionRotationDiff(&commonpb.Pose{OY: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, arm.PositionRotationDiff(&commonpb.Pose{OZ: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 1)
	test.That(t, arm.PositionRotationDiff(&commonpb.Pose{OX: 1, OY: 1, OZ: 1}, &commonpb.Pose{}), test.ShouldAlmostEqual, 3)
}

var pose = &commonpb.Pose{X: 1, Y: 2, Z: 3}

type mock struct {
	arm.Arm
	Name        string
	endPosCount int
	reconfCount int
}

func (m *mock) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	m.endPosCount++
	return pose, nil
}

func (m *mock) Close(ctx context.Context) error { m.reconfCount++; return nil }

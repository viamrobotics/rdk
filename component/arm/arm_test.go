package arm_test

import (
	"context"
	"math"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/sensor"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testArmName    = "arm1"
	testArmName2   = "arm2"
	failArmName    = "arm3"
	fakeArmName    = "arm4"
	missingArmName = "arm5"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[arm.Named(testArmName)] = &mock{Name: testArmName}
	deps[arm.Named(fakeArmName)] = "not an arm"
	return deps
}

func setupInjectRobot() *inject.Robot {
	arm1 := &mock{Name: testArmName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case arm.Named(testArmName):
			return arm1, nil
		case arm.Named(fakeArmName):
			return "not an arm", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named(testArmName), sensor.Named("sensor1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	a, err := arm.FromRobot(r, testArmName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := a.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	a, err := arm.FromDependencies(deps, testArmName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a, test.ShouldNotBeNil)

	pose1, err := a.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose1, test.ShouldResemble, pose)

	a, err = arm.FromDependencies(deps, fakeArmName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyTypeError(fakeArmName, "Arm", "string"))
	test.That(t, a, test.ShouldBeNil)

	a, err = arm.FromDependencies(deps, missingArmName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingArmName))
	test.That(t, a, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	a, err := arm.FromRobot(r, testArmName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a, test.ShouldNotBeNil)

	pose1, err := a.GetEndPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose1, test.ShouldResemble, pose)

	a, err = arm.FromRobot(r, fakeArmName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Arm", "string"))
	test.That(t, a, test.ShouldBeNil)

	a, err = arm.FromRobot(r, missingArmName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(arm.Named(missingArmName)))
	test.That(t, a, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := arm.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testArmName})
}

func TestStatusValid(t *testing.T) {
	status := &pb.Status{
		EndPosition:    pose,
		JointPositions: &pb.JointPositions{Degrees: []float64{1.1, 2.2, 3.3}},
		IsMoving:       true,
	}
	map1, err := protoutils.InterfaceToMap(status)
	test.That(t, err, test.ShouldBeNil)
	newStruct, err := structpb.NewStruct(map1)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{
			"end_position":    map[string]interface{}{"x": 1.0, "y": 2.0, "z": 3.0},
			"joint_positions": map[string]interface{}{"degrees": []interface{}{1.1, 2.2, 3.3}},
			"is_moving":       true,
		},
	)

	convMap := &pb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	_, err := arm.CreateStatus(context.Background(), "not an arm")
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalArm", "string"))

	status := &pb.Status{
		EndPosition:    pose,
		JointPositions: &pb.JointPositions{Degrees: []float64{1.1, 2.2, 3.3}},
		IsMoving:       true,
	}

	injectArm := &inject.Arm{}
	injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return pose, nil
	}
	injectArm.GetJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return &pb.JointPositions{Degrees: status.JointPositions.Degrees}, nil
	}
	injectArm.IsMovingFunc = func() bool {
		return true
	}

	t.Run("working", func(t *testing.T) {
		status1, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceSubtype := registry.ResourceSubtypeLookup(arm.Subtype)
		status2, err := resourceSubtype.Status(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("fail on GetJointPositions", func(t *testing.T) {
		errFail := errors.New("can't get joint positions")
		injectArm.GetJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
			return nil, errFail
		}
		_, err = arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("fail on GetEndPosition", func(t *testing.T) {
		errFail := errors.New("can't get position")
		injectArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
			return nil, errFail
		}
		_, err = arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeError, errFail)
	})
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
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalArm", nil))

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

func TestStop(t *testing.T) {
	actualArm1 := &mock{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualArm1.stopCount, test.ShouldEqual, 0)
	test.That(t, reconfArm1.(arm.Arm).Stop(context.Background()), test.ShouldBeNil)
	test.That(t, actualArm1.stopCount, test.ShouldEqual, 1)
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
	arm.LocalArm
	Name        string
	endPosCount int
	reconfCount int
	stopCount   int
}

func (m *mock) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	m.endPosCount++
	return pose, nil
}

func (m *mock) Stop(ctx context.Context) error {
	m.stopCount++
	return nil
}

func (m *mock) Close(ctx context.Context) error { m.reconfCount++; return nil }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

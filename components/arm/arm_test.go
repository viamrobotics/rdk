package arm_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
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
	deps[arm.Named(testArmName)] = &mockLocal{Name: testArmName}
	deps[arm.Named(fakeArmName)] = "not an arm"
	return deps
}

func setupInjectRobot() *inject.Robot {
	arm1 := &mockLocal{Name: testArmName}
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
	ret, err := a.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	a, err := arm.FromDependencies(deps, testArmName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a, test.ShouldNotBeNil)

	pose1, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose1, test.ShouldResemble, pose)

	a, err = arm.FromDependencies(deps, fakeArmName)
	test.That(t, err, test.ShouldBeError, arm.DependencyTypeError(fakeArmName, "string"))
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

	pose1, err := a.EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose1, test.ShouldResemble, pose)

	a, err = arm.FromRobot(r, fakeArmName)
	test.That(t, err, test.ShouldBeError, arm.NewUnimplementedInterfaceError("string"))
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
		EndPosition:    spatialmath.PoseToProtobuf(pose),
		JointPositions: &pb.JointPositions{Values: []float64{1.1, 2.2, 3.3}},
		IsMoving:       true,
	}
	newStruct, err := protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{
			"end_position":    map[string]interface{}{"o_z": 1.0, "x": 1.0, "y": 2.0, "z": 3.0},
			"joint_positions": map[string]interface{}{"values": []interface{}{1.1, 2.2, 3.3}},
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
	test.That(t, err, test.ShouldBeError, arm.NewUnimplementedLocalInterfaceError("string"))

	status := &pb.Status{
		EndPosition:    spatialmath.PoseToProtobuf(pose),
		JointPositions: &pb.JointPositions{Values: []float64{1.1, 2.2, 3.3}},
		IsMoving:       true,
	}

	injectArm := &inject.Arm{}
	injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return pose, nil
	}
	injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
		return &pb.JointPositions{Values: status.JointPositions.Values}, nil
	}
	injectArm.IsMovingFunc = func(context.Context) (bool, error) {
		return true, nil
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

	t.Run("not moving", func(t *testing.T) {
		injectArm.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}

		status2 := &pb.Status{
			EndPosition:    spatialmath.PoseToProtobuf(pose),
			JointPositions: &pb.JointPositions{Values: []float64{1.1, 2.2, 3.3}},
			IsMoving:       false,
		}
		status1, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status2)
	})

	t.Run("fail on JointPositions", func(t *testing.T) {
		errFail := errors.New("can't get joint positions")
		injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
			return nil, errFail
		}
		_, err = arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeError, errFail)
	})

	t.Run("fail on EndPosition", func(t *testing.T) {
		errFail := errors.New("can't get position")
		injectArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
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
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = arm.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, arm.NewUnimplementedInterfaceError(nil))

	reconfArm2, err := arm.WrapWithReconfigurable(reconfArm1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm2, test.ShouldEqual, reconfArm1)

	var actualArm2 arm.LocalArm = &mockLocal{Name: testArmName}
	reconfArm3, err := arm.WrapWithReconfigurable(actualArm2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	reconfArm4, err := arm.WrapWithReconfigurable(reconfArm3, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm4, test.ShouldResemble, reconfArm3)

	_, ok := reconfArm4.(arm.LocalArm)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestReconfigurableArm(t *testing.T) {
	actualArm1 := &mockLocal{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm1, test.ShouldNotBeNil)

	actualArm2 := &mockLocal{Name: testArmName2}
	reconfArm2, err := arm.WrapWithReconfigurable(actualArm2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm2, test.ShouldNotBeNil)
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 0)

	err = reconfArm1.Reconfigure(context.Background(), reconfArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm1, test.ShouldResemble, reconfArm2)
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 2)

	test.That(t, actualArm1.endPosCount, test.ShouldEqual, 0)
	test.That(t, actualArm2.endPosCount, test.ShouldEqual, 0)
	pose1, err := reconfArm1.(arm.Arm).EndPosition(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose1, test.ShouldResemble, pose)
	test.That(t, actualArm1.endPosCount, test.ShouldEqual, 0)
	test.That(t, actualArm2.endPosCount, test.ShouldEqual, 1)

	err = reconfArm1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfArm1, nil))

	actualArm3 := &mock{Name: failArmName}
	reconfArm3, err := arm.WrapWithReconfigurable(actualArm3, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm3, test.ShouldNotBeNil)

	actualArm4 := &mock{Name: testArmName2}
	reconfArm4, err := arm.WrapWithReconfigurable(actualArm4, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm4, test.ShouldNotBeNil)

	err = reconfArm3.Reconfigure(context.Background(), reconfArm4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfArm3, test.ShouldResemble, reconfArm4)
}

func TestStop(t *testing.T) {
	actualArm1 := &mockLocal{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualArm1.stopCount, test.ShouldEqual, 0)
	test.That(t, reconfArm1.(arm.Arm).Stop(context.Background(), nil), test.ShouldBeNil)
	test.That(t, actualArm1.stopCount, test.ShouldEqual, 1)
}

func TestClose(t *testing.T) {
	actualArm1 := &mockLocal{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfArm1), test.ShouldBeNil)
	test.That(t, actualArm1.reconfCount, test.ShouldEqual, 1)
}

func TestExtraOptions(t *testing.T) {
	actualArm1 := &mockLocal{Name: testArmName}
	reconfArm1, err := arm.WrapWithReconfigurable(actualArm1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualArm1.extra, test.ShouldEqual, nil)

	reconfArm1.(arm.Arm).EndPosition(context.Background(), map[string]interface{}{"foo": "bar"})
	test.That(t, actualArm1.extra, test.ShouldResemble, map[string]interface{}{"foo": "bar"})
}

func TestOOBArm(t *testing.T) {
	// instantiate OOB arm
	limits := make(map[string][]float64)
	limits["j1"] = []float64{1, -10}
	limits["j2"] = []float64{1, -10}
	limits["j3"] = []float64{1, -10}

	jPos := make(map[string]float64)
	jPos["j1"] = 10
	jPos["j2"] = 10
	jPos["j3"] = 10

	actualArm := &mockOOB{
		Name:      testArmName,
		JLimits:   limits,
		JPosition: jPos,
		Joints:    []string{"j1", "j2", "j3"},
	}

	t.Run("EndPosition and JointPositions work when OOB", func(t *testing.T) {
		pose1, err := actualArm.EndPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pose1, test.ShouldResemble, pose)

		positions1, err := actualArm.JointPositions(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		actualPositions := []float64{actualArm.JPosition["j1"], actualArm.JPosition["j2"], actualArm.JPosition["j3"]}
		test.That(t, positions1.GetValues(), test.ShouldResemble, actualPositions)
	})

	t.Run("MoveToPosition fails when OOB", func(t *testing.T) {
		err := actualArm.MoveToPosition(context.Background(), pose, &referenceframe.WorldState{}, nil)
		test.That(t, err.Error(), test.ShouldEqual, "10 needs to be within bounds [-10, 1] before MoveToPosition can work")
	})

	t.Run("MoveToJointPositions fails if more OOB", func(t *testing.T) {
		vals := []float64{11, 10, 10}
		err := actualArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: vals}, nil)
		test.That(t, err.Error(), test.ShouldEqual, "11 needs to be within bounds [-10, 10]")
	})

	t.Run("GoToInputs fails if more OOB", func(t *testing.T) {
		goal := []referenceframe.Input{{Value: 11}, {Value: 10}, {Value: 10}}
		err := actualArm.GoToInputs(context.Background(), goal)
		test.That(t, err.Error(), test.ShouldEqual, "-100 needs to be within bounds [-10, 10]")
	})

	t.Run("MoveToJointPositions works if more in bounds", func(t *testing.T) {
		vals := []float64{9, 10, 10}
		err := actualArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: vals}, nil)
		test.That(t, err, test.ShouldBeNil)
		actualPositions := []float64{actualArm.JPosition["j1"], actualArm.JPosition["j2"], actualArm.JPosition["j3"]}
		test.That(t, actualPositions, test.ShouldResemble, vals)
	})

	t.Run("MoveToJointPositions works if completely in bounds", func(t *testing.T) {
		vals := []float64{-5, -5, -5}
		err := actualArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: vals}, nil)
		test.That(t, err, test.ShouldBeNil)
		actualPositions := []float64{actualArm.JPosition["j1"], actualArm.JPosition["j2"], actualArm.JPosition["j3"]}
		test.That(t, actualPositions, test.ShouldResemble, vals)
	})

	t.Run("MoveToJointPositions fails if causes OOB from IB", func(t *testing.T) {
		vals := []float64{-15, -15, -15}
		err := actualArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: vals}, nil)
		test.That(t, err.Error(), test.ShouldEqual, "-15 needs to be within bounds [-10, 1]")
	})

	t.Run("MoveToPosition works when IB", func(t *testing.T) {
		err := actualArm.MoveToPosition(context.Background(), pose, &referenceframe.WorldState{}, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GoToInputs works when IB", func(t *testing.T) {
		goal := []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}}
		err := actualArm.GoToInputs(context.Background(), goal)
		test.That(t, err, test.ShouldBeNil)
		vals := []float64{0, 0, 0}
		actualPositions := []float64{actualArm.JPosition["j1"], actualArm.JPosition["j2"], actualArm.JPosition["j3"]}
		test.That(t, actualPositions, test.ShouldResemble, vals)
	})
}

var pose = spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})

type mockOOB struct {
	arm.Arm   // should this be arm.Arm?
	Name      string
	JLimits   map[string][]float64
	JPosition map[string]float64
	Joints    []string
}

func (m *mockOOB) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	return pose, nil
}

func (m *mockOOB) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	positionList := []float64{m.JPosition["j1"], m.JPosition["j2"], m.JPosition["j3"]}
	positions := pb.JointPositions{
		Values: positionList,
	}
	return &positions, nil
}

func (m *mockOOB) MoveToPosition(
	ctx context.Context,
	pose spatialmath.Pose,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	for jName := range m.JLimits {
		max := m.JLimits[jName][0]
		min := m.JLimits[jName][1]
		currPosition := m.JPosition[jName]
		if currPosition > max || currPosition < min {
			return fmt.Errorf("%v needs to be within bounds [%v, %v] before MoveToPosition can work", currPosition, min, max)
		}
	}
	return nil
}

func (m *mockOOB) MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions, extra map[string]interface{}) error {
	// check that joint positions are not out of bounds
	if err := m.checkDesiredJointPositions(positionDegs.Values); err != nil {
		return err
	}
	moveTo := positionDegs.Values
	m.JPosition["j1"] = moveTo[0]
	m.JPosition["j2"] = moveTo[1]
	m.JPosition["j3"] = moveTo[2]
	return nil
}

func (m *mockOOB) checkDesiredJointPositions(desiredJoints []float64) error {
	positions := []float64{m.JPosition["j1"], m.JPosition["j2"], m.JPosition["j3"]}
	for i, val := range desiredJoints {
		name := strings.Join([]string{"j", strconv.Itoa(i + 1)}, "")
		limits := m.JLimits[name]
		max := limits[0]
		min := limits[1]
		currPosition := positions[i]
		if currPosition > max {
			max = currPosition
		} else if currPosition < min {
			min = currPosition
		}
		if val > max || val < min {
			return fmt.Errorf("%v needs to be within bounds [%v, %v]", val, min, max)
		}
	}
	return nil
}

func (m *mockOOB) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	var vals []float64
	x := []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}}
	if x[0] == goal[0] && x[1] == goal[1] && x[2] == goal[2] {
		vals = []float64{0, 0, 0}
	} else {
		vals = []float64{-100, -100, -100}
	}
	if err := m.checkDesiredJointPositions(vals); err != nil {
		return err
	}
	m.JPosition["j1"] = vals[0]
	m.JPosition["j2"] = vals[1]
	m.JPosition["j3"] = vals[2]
	return nil
}

type mock struct {
	arm.Arm
	Name        string
	reconfCount int
}

func (m *mock) Close(ctx context.Context, extra map[string]interface{}) error {
	m.reconfCount++
	return nil
}

type mockLocal struct {
	arm.LocalArm
	Name        string
	endPosCount int
	reconfCount int
	stopCount   int
	extra       map[string]interface{}
}

func (m *mockLocal) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	m.endPosCount++
	m.extra = extra
	return pose, nil
}

func (m *mockLocal) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.stopCount++
	m.extra = extra
	return nil
}

func (m *mockLocal) Close(ctx context.Context) error { m.reconfCount++; return nil }

func (m *mockLocal) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

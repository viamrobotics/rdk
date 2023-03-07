package arm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	ur "go.viam.com/rdk/components/arm/universalrobots"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
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

	testPose := spatialmath.NewPose(
		r3.Vector{-802.801508917897990613710135, -248.284077946287368376943050, 9.115758604150467903082244},
		&spatialmath.R4AA{1.5810814917942602, 0.992515011486776, -0.0953988491934626, 0.07624310818669232},
	)
	status := &pb.Status{
		EndPosition:    spatialmath.PoseToProtobuf(testPose),
		JointPositions: &pb.JointPositions{Values: []float64{1.1, 2.2, 3.3, 1.1, 2.2, 3.3}},
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
	injectArm.ModelFrameFunc = func() referenceframe.Model {
		model, _ := ur.Model("ur5e")
		return model
	}

	t.Run("working", func(t *testing.T) {
		status1, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1.IsMoving, test.ShouldResemble, status.IsMoving)
		test.That(t, status1.JointPositions, test.ShouldResemble, status.JointPositions)
		pose1 := spatialmath.NewPoseFromProtobuf(status1.EndPosition)
		pose2 := spatialmath.NewPoseFromProtobuf(status.EndPosition)
		test.That(t, spatialmath.PoseAlmostEqualEps(pose1, pose2, 0.01), test.ShouldBeTrue)

		resourceSubtype := registry.ResourceSubtypeLookup(arm.Subtype)
		status2, err := resourceSubtype.Status(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)

		statusMap, err := protoutils.InterfaceToMap(status2)
		test.That(t, err, test.ShouldBeNil)

		endPosMap, err := protoutils.InterfaceToMap(statusMap["end_position"])
		test.That(t, err, test.ShouldBeNil)
		pose3 := spatialmath.NewPose(
			r3.Vector{endPosMap["x"].(float64), endPosMap["y"].(float64), endPosMap["z"].(float64)},
			&spatialmath.OrientationVectorDegrees{
				endPosMap["theta"].(float64), endPosMap["o_x"].(float64),
				endPosMap["o_y"].(float64), endPosMap["o_z"].(float64),
			},
		)
		test.That(t, spatialmath.PoseAlmostEqualEps(pose3, pose2, 0.01), test.ShouldBeTrue)

		moving := statusMap["is_moving"].(bool)
		test.That(t, moving, test.ShouldResemble, status.IsMoving)

		jPosFace := statusMap["joint_positions"].(map[string]interface{})["values"].([]interface{})
		jPos := []float64{
			jPosFace[0].(float64), jPosFace[1].(float64), jPosFace[2].(float64),
			jPosFace[3].(float64), jPosFace[4].(float64), jPosFace[5].(float64),
		}
		test.That(t, jPos, test.ShouldResemble, status.JointPositions.Values)
	})

	t.Run("not moving", func(t *testing.T) {
		injectArm.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}

		status2 := &pb.Status{
			EndPosition:    spatialmath.PoseToProtobuf(testPose),
			JointPositions: &pb.JointPositions{Values: []float64{1.1, 2.2, 3.3, 1.1, 2.2, 3.3}},
			IsMoving:       false,
		}
		status1, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1.IsMoving, test.ShouldResemble, status2.IsMoving)
		test.That(t, status1.JointPositions, test.ShouldResemble, status2.JointPositions)
		pose1 := spatialmath.NewPoseFromProtobuf(status1.EndPosition)
		pose2 := spatialmath.NewPoseFromProtobuf(status2.EndPosition)
		test.That(t, spatialmath.PoseAlmostEqualEps(pose1, pose2, 0.01), test.ShouldBeTrue)
	})

	t.Run("fail on JointPositions", func(t *testing.T) {
		errFail := errors.New("can't get joint positions")
		injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
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
	logger := golog.NewTestLogger(t)
	cfg := config.Component{
		Name:  arm.Subtype.String(),
		Model: resource.NewDefaultModel("ur5e"),
		ConvertedAttributes: &fake.AttrConfig{
			ArmModel: "ur5e",
		},
	}

	notReal, err := fake.NewArm(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	injectedArm := &inject.Arm{
		LocalArm: notReal,
	}

	jPositions := pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}
	injectedArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
		return &jPositions, nil
	}

	// instantiate OOB arm
	positions, err := injectedArm.JointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, &jPositions)

	t.Run("CreateStatus errors when OOB", func(t *testing.T) {
		status, err := arm.CreateStatus(context.Background(), injectedArm)
		test.That(t, status, test.ShouldBeNil)
		stringCheck := "joint 0 input out of bounds, input 12.56637 needs to be within range [6.28319 -6.28319]"
		test.That(t, err.Error(), test.ShouldEqual, stringCheck)
	})

	t.Run("EndPosition works when OOB", func(t *testing.T) {
		jPositions := pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}
		pose, err := motionplan.ComputeOOBPosition(injectedArm.ModelFrame(), &jPositions)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pose, test.ShouldNotBeNil)
	})

	t.Run("MoveToPosition fails when OOB", func(t *testing.T) {
		pose = spatialmath.NewPoseFromPoint(r3.Vector{200, 200, 200})
		err := arm.Move(context.Background(), &inject.Robot{}, injectedArm, pose, &referenceframe.WorldState{})
		u := "cartesian movements are not allowed when arm joints are out of bounds"
		v := "joint 0 input out of bounds, input 12.56637 needs to be within range [6.28319 -6.28319]"
		s := strings.Join([]string{u, v}, ": ")
		test.That(t, err.Error(), test.ShouldEqual, s)
	})

	t.Run("MoveToJointPositions fails if more OOB", func(t *testing.T) {
		vals := []float64{0, 0, 0, 0, 0, 800}
		err := arm.CheckDesiredJointPositions(context.Background(), injectedArm, vals)
		test.That(t, err.Error(), test.ShouldEqual, "joint 5 needs to be within range [-360, 720] and cannot be moved to 800")
	})

	t.Run("GoToInputs fails if more OOB", func(t *testing.T) {
		goal := []referenceframe.Input{{Value: 11}, {Value: 10}, {Value: 10}, {Value: 11}, {Value: 10}, {Value: 10}}
		model := injectedArm.LocalArm.ModelFrame()
		positionDegs := model.ProtobufFromInput(goal)
		err := arm.CheckDesiredJointPositions(context.Background(), injectedArm, positionDegs.Values)
		test.That(t, err.Error(), test.ShouldEqual, "joint 0 needs to be within range [-360, 360] and cannot be moved to 630.2535746439056")
	})

	t.Run("MoveToJointPositions works if more in bounds", func(t *testing.T) {
		vals := []float64{0, 0, 0, 0, 0, 400}
		err := arm.CheckDesiredJointPositions(context.Background(), injectedArm, vals)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("MoveToJointPositions works if completely in bounds", func(t *testing.T) {
		vals := []float64{0, 0, 0, 0, 0, 0}
		err := injectedArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: vals}, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("MoveToJointPositions fails if causes OOB from IB", func(t *testing.T) {
		vals := []float64{0, 0, 0, 0, 0, 400}
		err := injectedArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: vals}, nil)
		output := "joint 5 needs to be within range [-360, 360] and cannot be moved to 400"
		test.That(t, err.Error(), test.ShouldEqual, output)
	})

	t.Run("MoveToPosition works when IB", func(t *testing.T) {
		pose = spatialmath.NewPoseFromPoint(r3.Vector{200, 200, 200})
		err := injectedArm.MoveToPosition(context.Background(), pose, &referenceframe.WorldState{}, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GoToInputs works when IB", func(t *testing.T) {
		goal := []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}, {Value: 0}, {Value: 0}, {Value: 0}}
		err := injectedArm.GoToInputs(context.Background(), goal)
		test.That(t, err, test.ShouldBeNil)
	})
}

var pose = spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})

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

func TestXArm6Locations(t *testing.T) {
	// check the exact values/locations of arm geometries at a couple different poses
	logger := golog.NewTestLogger(t)
	cfg := config.Component{
		Name:  arm.Subtype.String(),
		Model: resource.NewDefaultModel("fake"),
		ConvertedAttributes: &fake.AttrConfig{
			ArmModel: "xArm6",
		},
	}
	notReal, err := fake.NewArm(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("home location check", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			402.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			102.997474683058328537299531,
			0.000000000000000000000000,
			502.002525316941671462700469,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			131.000000000000000000000000,
			0.000000000000000000000000,
			274.000000000000000000000000,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			206.000000000000000000000000,
			10.000000000000000000000000,
			141.500000000000000000000000,
		}

		in := make([]referenceframe.Input, 6)
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	t.Run("location check1", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			-13.477511247321800169629569,
			0.000000000000000000000000,
			401.325562312533520525903441,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			83.174566602088916056345624,
			0.000000000000000000000000,
			516.742582359123957758129109,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			155.916014884677508689492242,
			-0.000000000000000289509481,
			298.848170597876446663576644,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			259.050559200277916716004256,
			18.072894555453149934010071,
			192.543551746158527748775668,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	t.Run("location check2", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:base_top"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			160.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm"] = r3.Vector{
			-26.820359657333263214695762,
			0.000000000000000000000000,
			399.308988008567553151806351,
		}
		checkMap["rdk:component:arm:upper_forearm"] = r3.Vector{
			60.777555075062835499011271,
			0.000000000000000000000000,
			530.142781900699674224597402,
		}
		checkMap["rdk:component:arm:lower_forearm"] = r3.Vector{
			175.357954329185588449036004,
			0.000000000000000363462780,
			331.043246286488795249169925,
		}
		checkMap["rdk:component:arm:wrist_link"] = r3.Vector{
			297.258065257027055849903263,
			27.068045067389423508075197,
			256.363984524505951867467957,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
}

func TestUR5ELocations(t *testing.T) {
	// check the exact values/locations of arm geometries at a couple different poses
	logger := golog.NewTestLogger(t)
	cfg := config.Component{
		Name:  arm.Subtype.String(),
		Model: resource.NewDefaultModel("fake"),
		ConvertedAttributes: &fake.AttrConfig{
			ArmModel: "ur5e",
		},
	}
	notReal, err := fake.NewArm(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("home location check", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:wrist_1_link"] = r3.Vector{
			-817.200000000000045474735089,
			-66.649999999999948840923025,
			162.500000000000000000000000,
		}
		checkMap["rdk:component:arm:wrist_2_link"] = r3.Vector{
			-817.200000000000045474735089,
			-133.300000000000011368683772,
			112.650000000000005684341886,
		}
		checkMap["rdk:component:arm:ee_link"] = r3.Vector{
			-817.200000000000045474735089,
			-183.149999999999920419213595,
			62.799999999999940314410196,
		}
		checkMap["rdk:component:arm:base_link"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			120.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm_link"] = r3.Vector{
			-212.500000000000000000000000,
			-130.000000000000000000000000,
			162.499999999999971578290570,
		}
		checkMap["rdk:component:arm:forearm_link"] = r3.Vector{
			-621.100000000000022737367544,
			0.000000000000000000000000,
			162.500000000000000000000000,
		}

		in := make([]referenceframe.Input, 6)
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check1", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:wrist_2_link"] = r3.Vector{
			-821.990564374563746241619810,
			-133.300000000000068212102633,
			235.223789629813609280972742,
		}
		checkMap["rdk:component:arm:wrist_1_link"] = r3.Vector{
			-807.258882072496135151595809,
			-66.650000000000062527760747,
			282.847313612725088205479551,
		}
		checkMap["rdk:component:arm:ee_link"] = r3.Vector{
			-831.967827564655408423277549,
			-182.900957639109606134297792,
			186.129551469731126189799397,
		}
		checkMap["rdk:component:arm:base_link"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			120.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm_link"] = r3.Vector{
			-211.438385121580523673401331,
			-130.000000000000028421709430,
			183.714601037450989906574250,
		}
		checkMap["rdk:component:arm:forearm_link"] = r3.Vector{
			-615.067826157828449140652083,
			-0.000000000000000000000000,
			243.888257843813590852732887,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}, {Value: -0.1}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
	//nolint:dupl
	t.Run("location check2", func(t *testing.T) {
		checkMap := make(map[string]r3.Vector)
		checkMap["rdk:component:arm:wrist_1_link"] = r3.Vector{
			-777.768417430459294337197207,
			-66.650000000000005684341886,
			399.664339441353661186440149,
		}
		checkMap["rdk:component:arm:wrist_2_link"] = r3.Vector{
			-805.915844729201694462972227,
			-133.300000000000011368683772,
			358.521359038106197658635210,
		}
		checkMap["rdk:component:arm:ee_link"] = r3.Vector{
			-825.889423644316707395773847,
			-182.156318905385916195882601,
			311.786348089814850936818402,
		}
		checkMap["rdk:component:arm:base_link"] = r3.Vector{
			0.000000000000000000000000,
			0.000000000000000000000000,
			120.000000000000000000000000,
		}
		checkMap["rdk:component:arm:upper_arm_link"] = r3.Vector{
			-208.264147791263866338340449,
			-130.000000000000028421709430,
			204.717232793950500990831642,
		}
		checkMap["rdk:component:arm:forearm_link"] = r3.Vector{
			-597.148356506493314554973040,
			-0.000000000000000000000000,
			323.299402514627388427470578,
		}

		in := []referenceframe.Input{{Value: 0}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}, {Value: -0.2}}
		geoms, err := notReal.ModelFrame().Geometries(in)
		test.That(t, err, test.ShouldBeNil)
		geomMap := geoms.Geometries()
		b := locationCheckTestHelper(geomMap, checkMap)
		test.That(t, b, test.ShouldBeTrue)
	})
}

func locationCheckTestHelper(geomList []spatialmath.Geometry, checkMap map[string]r3.Vector) bool {
	for _, g := range geomList {
		vecCheck := checkMap[g.Label()]
		vecActual := g.Pose().Point()
		if !spatialmath.R3VectorAlmostEqual(vecCheck, vecActual, 1e-2) {
			return false
		}
	}
	return true
}

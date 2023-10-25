package arm_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	ur "go.viam.com/rdk/components/arm/universalrobots"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testArmName    = "arm1"
	testArmName2   = "arm2"
	failArmName    = "arm3"
	missingArmName = "arm4"
)

var pose = spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3})

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
	successfulPose := spatialmath.NewPose(
		r3.Vector{-802.801508917897990613710135, -248.284077946287368376943050, 9.115758604150467903082244},
		&spatialmath.R4AA{1.5810814917942602, 0.992515011486776, -0.0953988491934626, 0.07624310818669232},
	)
	successfulStatus := &pb.Status{
		EndPosition:    spatialmath.PoseToProtobuf(successfulPose),
		JointPositions: &pb.JointPositions{Values: []float64{1.1, 2.2, 3.3, 1.1, 2.2, 3.3}},
		IsMoving:       true,
	}

	injectArm := &inject.Arm{}

	//nolint:unparam
	successfulJointPositionsFunc := func(context.Context, map[string]interface{}) (*pb.JointPositions, error) {
		return successfulStatus.JointPositions, nil
	}

	successfulIsMovingFunc := func(context.Context) (bool, error) {
		return true, nil
	}

	successfulModelFrameFunc := func() referenceframe.Model {
		model, _ := ur.MakeModelFrame("ur5e")
		return model
	}

	t.Run("working", func(t *testing.T) {
		injectArm.JointPositionsFunc = successfulJointPositionsFunc
		injectArm.IsMovingFunc = successfulIsMovingFunc
		injectArm.ModelFrameFunc = successfulModelFrameFunc

		expectedPose := successfulPose
		expectedStatus := successfulStatus

		actualStatus, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualStatus.IsMoving, test.ShouldEqual, expectedStatus.IsMoving)
		test.That(t, actualStatus.JointPositions, test.ShouldResemble, expectedStatus.JointPositions)

		actualPose := spatialmath.NewPoseFromProtobuf(actualStatus.EndPosition)
		test.That(t, spatialmath.PoseAlmostEqualEps(actualPose, expectedPose, 0.01), test.ShouldBeTrue)

		resourceAPI, ok, err := resource.LookupAPIRegistration[arm.Arm](arm.API)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ok, test.ShouldBeTrue)
		statusInterface, err := resourceAPI.Status(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)

		statusMap, err := protoutils.InterfaceToMap(statusInterface)
		test.That(t, err, test.ShouldBeNil)

		endPosMap, err := protoutils.InterfaceToMap(statusMap["end_position"])
		test.That(t, err, test.ShouldBeNil)
		actualPose = spatialmath.NewPose(
			r3.Vector{endPosMap["x"].(float64), endPosMap["y"].(float64), endPosMap["z"].(float64)},
			&spatialmath.OrientationVectorDegrees{
				endPosMap["theta"].(float64), endPosMap["o_x"].(float64),
				endPosMap["o_y"].(float64), endPosMap["o_z"].(float64),
			},
		)
		test.That(t, spatialmath.PoseAlmostEqualEps(actualPose, expectedPose, 0.01), test.ShouldBeTrue)

		moving := statusMap["is_moving"].(bool)
		test.That(t, moving, test.ShouldEqual, expectedStatus.IsMoving)

		jPosFace := statusMap["joint_positions"].(map[string]interface{})["values"].([]interface{})
		actualJointPositions := []float64{
			jPosFace[0].(float64), jPosFace[1].(float64), jPosFace[2].(float64),
			jPosFace[3].(float64), jPosFace[4].(float64), jPosFace[5].(float64),
		}
		test.That(t, actualJointPositions, test.ShouldResemble, expectedStatus.JointPositions.Values)
	})

	t.Run("not moving", func(t *testing.T) {
		injectArm.JointPositionsFunc = successfulJointPositionsFunc
		injectArm.ModelFrameFunc = successfulModelFrameFunc

		injectArm.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}

		expectedPose := successfulPose
		expectedStatus := &pb.Status{
			EndPosition:    successfulStatus.EndPosition, //nolint:govet
			JointPositions: successfulStatus.JointPositions,
			IsMoving:       false,
		}

		actualStatus, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualStatus.IsMoving, test.ShouldEqual, expectedStatus.IsMoving)
		test.That(t, actualStatus.JointPositions, test.ShouldResemble, expectedStatus.JointPositions)
		actualPose := spatialmath.NewPoseFromProtobuf(actualStatus.EndPosition)
		test.That(t, spatialmath.PoseAlmostEqualEps(actualPose, expectedPose, 0.01), test.ShouldBeTrue)
	})

	t.Run("fail on JointPositions", func(t *testing.T) {
		injectArm.IsMovingFunc = successfulIsMovingFunc
		injectArm.ModelFrameFunc = successfulModelFrameFunc

		errFail := errors.New("can't get joint positions")
		injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
			return nil, errFail
		}

		actualStatus, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeError, errFail)
		test.That(t, actualStatus, test.ShouldBeNil)
	})

	t.Run("nil JointPositions", func(t *testing.T) {
		injectArm.IsMovingFunc = successfulIsMovingFunc
		injectArm.ModelFrameFunc = successfulModelFrameFunc

		injectArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
			return nil, nil //nolint:nilnil
		}

		expectedStatus := &pb.Status{
			EndPosition:    nil,
			JointPositions: nil,
			IsMoving:       successfulStatus.IsMoving,
		}

		actualStatus, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualStatus.EndPosition, test.ShouldEqual, expectedStatus.EndPosition)
		test.That(t, actualStatus.JointPositions, test.ShouldEqual, expectedStatus.JointPositions)
		test.That(t, actualStatus.IsMoving, test.ShouldEqual, expectedStatus.IsMoving)
	})

	t.Run("nil model frame", func(t *testing.T) {
		injectArm.IsMovingFunc = successfulIsMovingFunc
		injectArm.JointPositionsFunc = successfulJointPositionsFunc

		injectArm.ModelFrameFunc = func() referenceframe.Model {
			return nil
		}

		expectedStatus := &pb.Status{
			EndPosition:    nil,
			JointPositions: successfulStatus.JointPositions,
			IsMoving:       successfulStatus.IsMoving,
		}

		actualStatus, err := arm.CreateStatus(context.Background(), injectArm)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualStatus.EndPosition, test.ShouldEqual, expectedStatus.EndPosition)
		test.That(t, actualStatus.JointPositions, test.ShouldResemble, expectedStatus.JointPositions)
		test.That(t, actualStatus.IsMoving, test.ShouldEqual, expectedStatus.IsMoving)
	})
}

func TestOOBArm(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("ur5e"),
		ConvertedAttributes: &fake.Config{
			ArmModel: "ur5e",
		},
	}

	notReal, err := fake.NewArm(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	injectedArm := &inject.Arm{
		Arm: notReal,
	}

	jPositions := pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}
	injectedArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
		return &jPositions, nil
	}

	// instantiate OOB arm
	positions, err := injectedArm.JointPositions(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, positions, test.ShouldResemble, &jPositions)

	t.Run("EndPosition works when OOB", func(t *testing.T) {
		jPositions := pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}
		pose, err := motionplan.ComputeOOBPosition(injectedArm.ModelFrame(), &jPositions)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pose, test.ShouldNotBeNil)
	})

	t.Run("Move fails when OOB", func(t *testing.T) {
		pose = spatialmath.NewPoseFromPoint(r3.Vector{200, 200, 200})
		err := arm.Move(context.Background(), logger, injectedArm, pose)
		u := "cartesian movements are not allowed when arm joints are out of bounds"
		v := "joint 0 input out of bounds, input 12.56637 needs to be within range [6.28319 -6.28319]"
		s := strings.Join([]string{u, v}, ": ")
		test.That(t, err.Error(), test.ShouldEqual, s)
	})

	t.Run("MoveToJointPositions fails if more OOB", func(t *testing.T) {
		vals := &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 800}}
		err := arm.CheckDesiredJointPositions(context.Background(), injectedArm, vals)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(
			t,
			err.Error(),
			test.ShouldEqual,
			"joint 5 needs to be within range [-6.283185307179586, 12.566370614359172] and cannot be moved to 13.962634015954636",
		)
	})

	t.Run("GoToInputs fails if more OOB", func(t *testing.T) {
		goal := []referenceframe.Input{{Value: 11}, {Value: 10}, {Value: 10}, {Value: 11}, {Value: 10}, {Value: 10}}
		model := injectedArm.Arm.ModelFrame()
		positionDegs := model.ProtobufFromInput(goal)
		err := arm.CheckDesiredJointPositions(context.Background(), injectedArm, positionDegs)
		test.That(
			t,
			err.Error(),
			test.ShouldEqual,
			"joint 0 needs to be within range [-6.283185307179586, 6.283185307179586] and cannot be moved to 11.000000000000002",
		)
	})

	t.Run("MoveToJointPositions works if more in bounds", func(t *testing.T) {
		vals := &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 400}}
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
		output := "joint 5 needs to be within range [-6.283185307179586, 6.283185307179586] and cannot be moved to 6.981317007977318"
		test.That(t, err.Error(), test.ShouldEqual, output)
	})

	t.Run("MoveToPosition works when IB", func(t *testing.T) {
		homePose, err := injectedArm.EndPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		testLinearMove := r3.Vector{homePose.Point().X + 20, homePose.Point().Y, homePose.Point().Z}
		testPose := spatialmath.NewPoseFromPoint(testLinearMove)
		err = injectedArm.MoveToPosition(context.Background(), testPose, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("GoToInputs works when IB", func(t *testing.T) {
		goal := []referenceframe.Input{{Value: 0}, {Value: 0}, {Value: 0}, {Value: 0}, {Value: 0}, {Value: 0}}
		err := injectedArm.GoToInputs(context.Background(), goal)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestXArm6Locations(t *testing.T) {
	// check the exact values/locations of arm geometries at a couple different poses
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &fake.Config{
			ArmModel: "xArm6",
		},
	}

	notReal, err := fake.NewArm(context.Background(), nil, cfg, logger)
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
			-27.500000000000000000000000,
			274.200000000000000000000000,
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
	//nolint:dupl
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
			158.566974381217988820935716,
			-27.362614545145717670493468,
			299.589614460540474283334333,
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
	//nolint:dupl
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
			180.312201371473605604478507,
			-26.951830890634148829576588,
			333.355009225598280409030849,
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
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &fake.Config{
			ArmModel: "ur5e",
		},
	}

	notReal, err := fake.NewArm(context.Background(), nil, cfg, logger)
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

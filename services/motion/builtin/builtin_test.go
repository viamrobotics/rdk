package builtin

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/go-viper/mapstructure/v2"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/encoding/protojson"

	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

func setupMotionServiceFromConfig(t *testing.T, configFilename string) (motion.Service, func()) {
	t.Helper()
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger, nil)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, nil, logger)
	test.That(t, err, test.ShouldBeNil)
	svc, err := motion.FromProvider(myRobot, "builtin")
	test.That(t, err, test.ShouldBeNil)
	return svc, func() {
		myRobot.Close(context.Background())
	}
}

func TestMoveFailures(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")
	defer teardown()
	ctx := context.Background()
	t.Run("fail on not finding gripper", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("fakeGripper", spatialmath.NewPoseFromPoint(r3.Vector{X: 10.0, Y: 10.0, Z: 10.0}))
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: "fake", Destination: grabPose})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on nil destination", func(t *testing.T) {
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: "arm1"})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on disconnected supplemental frames in world state", func(t *testing.T) {
		testPose := spatialmath.NewPose(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)
		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame("noParent", testPose, "frame2", nil),
		}
		worldState, err := referenceframe.NewWorldState(nil, transforms)
		test.That(t, err, test.ShouldBeNil)
		poseInFrame := referenceframe.NewPoseInFrame("frame2", spatialmath.NewZeroPose())
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: "arm1", Destination: poseInFrame, WorldState: worldState})
		test.That(t, err, test.ShouldBeError, referenceframe.NewParentFrameMissingError("frame2", "noParent"))
	})
}

func TestArmMove(t *testing.T) {
	var err error
	ctx := context.Background()

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: "pieceGripper", Destination: grabPose})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when mobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceArm", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: "pieceArm", Destination: grabPose})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when immobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: "pieceGripper", Destination: grabPose})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		testPose := spatialmath.NewPose(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)

		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame(referenceframe.World, testPose, "testFrame2", nil),
			referenceframe.NewLinkInFrame("pieceArm", testPose, "testFrame", nil),
		}

		worldState, err := referenceframe.NewWorldState(nil, transforms)
		test.That(t, err, test.ShouldBeNil)
		grabPose := referenceframe.NewPoseInFrame("testFrame2", spatialmath.NewPoseFromPoint(r3.Vector{X: -20, Y: -130, Z: -40}))
		moveReq := motion.MoveReq{ComponentName: "pieceGripper", Destination: grabPose, WorldState: worldState}
		_, err = ms.Move(context.Background(), moveReq)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestArmMoveWithObstacles(t *testing.T) {
	t.Run("check a movement that should not succeed due to obstacles", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		testPose1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 370})
		testPose2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 300, Y: 300, Z: -3500})
		_ = testPose2
		grabPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -600, Y: -400, Z: 460}))
		obsMsgs := []*commonpb.GeometriesInFrame{
			{
				ReferenceFrame: "world",
				Geometries: []*commonpb.Geometry{
					{
						Center: spatialmath.PoseToProtobuf(testPose2),
						GeometryType: &commonpb.Geometry_Box{
							Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{
								X: 20,
								Y: 40,
								Z: 40,
							}},
						},
					},
				},
			},
			{
				ReferenceFrame: "world",
				Geometries: []*commonpb.Geometry{
					{
						Center: spatialmath.PoseToProtobuf(testPose1),
						GeometryType: &commonpb.Geometry_Box{
							Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{
								X: 2000,
								Y: 2000,
								Z: 20,
							}},
						},
					},
				},
			},
		}
		worldState, err := referenceframe.WorldStateFromProtobuf(&commonpb.WorldState{Obstacles: obsMsgs})
		test.That(t, err, test.ShouldBeNil)
		_, err = ms.Move(
			context.Background(),
			motion.MoveReq{ComponentName: "pieceArm", Destination: grabPose, WorldState: worldState},
		)
		// This fails due to a large obstacle being in the way
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestMultiplePieces(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/fake_tomato.json")
	defer teardown()
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: -0, Y: -30, Z: -50}))
	_, err = ms.Move(context.Background(), motion.MoveReq{ComponentName: "gr", Destination: grabPose})
	test.That(t, err, test.ShouldBeNil)
}

func TestGetPose(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")
	defer teardown()

	pose, err := ms.GetPose(context.Background(), "gantry1", "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 1.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), "arm1", "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), "arm1", "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 500)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), "gantry1", "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), "arm1", "arm1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "arm1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	testPose := spatialmath.NewPoseFromOrientation(&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.})
	transforms := []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame(referenceframe.World, testPose, "testFrame", nil),
		referenceframe.NewLinkInFrame("testFrame", testPose, "testFrame2", nil),
	}

	pose, err = ms.GetPose(context.Background(), "arm1", "testFrame2", transforms, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, -501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, -300)
	test.That(t, pose.Pose().Orientation().AxisAngles().RX, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().RY, test.ShouldEqual, -1)
	test.That(t, pose.Pose().Orientation().AxisAngles().RZ, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().Theta, test.ShouldAlmostEqual, math.Pi)

	transforms = []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame("noParent", testPose, "testFrame", nil),
	}
	pose, err = ms.GetPose(context.Background(), "arm1", "testFrame", transforms, map[string]interface{}{})
	test.That(t, err, test.ShouldBeError, referenceframe.NewParentFrameMissingError("testFrame", "noParent"))
	test.That(t, pose, test.ShouldBeNil)
}

func TestDoCommand(t *testing.T) {
	ctx := context.Background()
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{1000, 1000, 1000}), r3.Vector{1, 1, 1}, "box")
	test.That(t, err, test.ShouldBeNil)
	geometries := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame("world", []spatialmath.Geometry{box})}
	worldState, err := referenceframe.NewWorldState(geometries, nil)
	test.That(t, err, test.ShouldBeNil)
	moveReq := motion.MoveReq{
		ComponentName: "pieceGripper",
		WorldState:    worldState,
		Destination:   referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50})),
		Extra:         nil,
	}

	// need to simulate what happens when the DoCommand message is serialized/deserialized into proto
	doOverWire := func(ms motion.Service, cmd map[string]interface{}) (map[string]interface{}, error) {
		command, err := protoutils.StructToStructPb(cmd)
		test.That(t, err, test.ShouldBeNil)
		resp, err := ms.DoCommand(ctx, command.AsMap())
		if err != nil {
			return map[string]interface{}{}, err
		}
		respProto, err := protoutils.StructToStructPb(resp)
		test.That(t, err, test.ShouldBeNil)
		return respProto.AsMap(), nil
	}

	testDoPlan := func(moveReq motion.MoveReq) (motionplan.Trajectory, error) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()

		// format the command to send DoCommand
		proto, err := moveReq.ToProto(ms.Name().Name)
		test.That(t, err, test.ShouldBeNil)
		bytes, err := protojson.Marshal(proto)
		test.That(t, err, test.ShouldBeNil)
		cmd := map[string]interface{}{DoPlan: string(bytes)}

		// simulate going over the wire
		respMap, err := doOverWire(ms, cmd)
		if err != nil {
			return nil, err
		}
		resp, ok := respMap[DoPlan]
		test.That(t, ok, test.ShouldBeTrue)

		// the client will need to decode the response still
		var trajectory motionplan.Trajectory
		err = mapstructure.Decode(resp, &trajectory)
		return trajectory, err
	}

	t.Run("DoPlan", func(t *testing.T) {
		trajectory, err := testDoPlan(moveReq)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(trajectory), test.ShouldEqual, 2)
	})

	t.Run("DoExectute", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()

		plan, err := ms.(*builtIn).plan(ctx, moveReq, logger)
		test.That(t, err, test.ShouldBeNil)

		// format the command to sent DoCommand
		cmd := map[string]interface{}{DoExecute: plan.Trajectory()}

		// simulate going over the wire
		respMap, err := doOverWire(ms, cmd)
		test.That(t, err, test.ShouldBeNil)
		resp, ok := respMap[DoExecute]
		test.That(t, ok, test.ShouldBeTrue)

		// the client will need to decode the response still
		test.That(t, resp, test.ShouldBeTrue)
	})
	t.Run("DoExecuteCheckStart", func(t *testing.T) {
		// generate a separate trajectory plan first. that way this state will not be affected by future executions
		trajectory, err := testDoPlan(moveReq)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(trajectory), test.ShouldEqual, 2)

		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()

		// format the command to sent DoCommand
		cmd := map[string]interface{}{DoExecute: trajectory, DoExecuteCheckStart: "not a float"}

		// simulate going over the wire
		respMap, err := doOverWire(ms, cmd)
		test.That(t, err, test.ShouldBeNil)
		resp, ok := respMap[DoExecute]
		test.That(t, ok, test.ShouldBeTrue)

		// the client will need to decode the response still
		test.That(t, resp, test.ShouldBeTrue)
		test.That(t, respMap[DoExecuteCheckStart], test.ShouldEqual, "resource at starting location")

		start := trajectory[0]["pieceArm"]
		end := trajectory[len(trajectory)-1]["pieceArm"]
		// do it again
		respMap, err = doOverWire(ms, cmd)
		test.That(t, err, test.ShouldBeError,
			fmt.Errorf("component %v is not within %v of the current position. Expected inputs %v current inputs %v",
				"pieceArm", defaultExecuteEpsilon, start, end))
		test.That(t, respMap, test.ShouldBeEmpty)
	})
}

func TestMultiWaypointPlanning(t *testing.T) {
	ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	defer teardown()
	ctx := context.Background()

	// Helper function to extract plan from Move call using DoCommand
	getPlanFromMove := func(t *testing.T, req motion.MoveReq) motionplan.Trajectory {
		t.Helper()
		// Convert MoveReq to proto format for DoCommand
		moveReqProto, err := req.ToProto("")
		test.That(t, err, test.ShouldBeNil)
		bytes, err := protojson.Marshal(moveReqProto)
		test.That(t, err, test.ShouldBeNil)

		resp, err := ms.DoCommand(ctx, map[string]interface{}{
			DoPlan: string(bytes),
		})
		test.That(t, err, test.ShouldBeNil)

		plan, ok := resp[DoPlan].(motionplan.Trajectory)
		test.That(t, ok, test.ShouldBeTrue)
		return plan
	}

	t.Run("plan through multiple pose waypoints", func(t *testing.T) {
		// Define waypoints as poses relative to world frame
		waypoint1 := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -180, Z: 30}))
		waypoint2 := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -190, Z: 30}))
		finalPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -200, Z: 30}))

		wp1State := armplanning.NewPlanState(referenceframe.FrameSystemPoses{"pieceGripper": waypoint1}, nil)
		wp2State := armplanning.NewPlanState(referenceframe.FrameSystemPoses{"pieceGripper": waypoint2}, nil)

		moveReq := motion.MoveReq{
			ComponentName: "pieceGripper",
			Destination:   finalPose,
			Extra: map[string]interface{}{
				"waypoints":   []interface{}{wp1State.Serialize(), wp2State.Serialize()},
				"smooth_iter": 5,
			},
		}

		plan := getPlanFromMove(t, moveReq)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

		// Verify start configuration matches current robot state
		fsInputs, err := ms.(*builtIn).fsService.CurrentInputs(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, plan[0], test.ShouldResemble, fsInputs)

		// Verify final pose
		frameSys, err := framesystem.NewFromService(ctx, ms.(*builtIn).fsService, nil)
		test.That(t, err, test.ShouldBeNil)

		finalConfig := plan[len(plan)-1]
		finalPoseInWorld, err := frameSys.Transform(finalConfig.ToLinearInputs(),
			referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewZeroPose()),
			"world")
		test.That(t, err, test.ShouldBeNil)
		plannedPose := finalPoseInWorld.(*referenceframe.PoseInFrame).Pose()
		test.That(t, spatialmath.PoseAlmostEqualEps(plannedPose, finalPose.Pose(), .01), test.ShouldBeTrue)
	})

	t.Run("plan through mixed pose and configuration waypoints", func(t *testing.T) {
		// Define specific arm configuration for first waypoint
		armConfig := []float64{0.2, 0.3, 0.4, 0.5, 0.6, 0.7}
		wp1State := armplanning.NewPlanState(nil, referenceframe.FrameSystemInputs{
			"pieceArm": armConfig,
		})

		// Define pose for second waypoint
		intermediatePose := spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -190, Z: 30})
		wp2State := armplanning.NewPlanState(
			referenceframe.FrameSystemPoses{"pieceGripper": referenceframe.NewPoseInFrame("world", intermediatePose)},
			nil,
		)

		finalPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -180, Z: 34}))

		moveReq := motion.MoveReq{
			ComponentName: "pieceGripper",
			Destination:   finalPose,
			Extra: map[string]interface{}{
				"waypoints":   []interface{}{wp1State.Serialize(), wp2State.Serialize()},
				"smooth_iter": 5,
			},
		}

		plan := getPlanFromMove(t, moveReq)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

		// Find configuration closest to first waypoint
		foundMatchingConfig := false
		for _, config := range plan {
			if armInputs, ok := config["pieceArm"]; ok {
				// Check if this configuration matches our waypoint within some epsilon
				matches := true
				for i, val := range armInputs {
					if math.Abs(val-armConfig[i]) > 1e-3 {
						matches = false
						break
					}
				}
				if matches {
					foundMatchingConfig = true
					break
				}
			}
		}
		test.That(t, foundMatchingConfig, test.ShouldBeTrue)

		// Verify final pose
		frameSys, err := framesystem.NewFromService(ctx, ms.(*builtIn).fsService, nil)
		test.That(t, err, test.ShouldBeNil)

		finalConfig := plan[len(plan)-1]
		finalPoseInWorld, err := frameSys.Transform(finalConfig.ToLinearInputs(),
			referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewZeroPose()),
			"world")
		test.That(t, err, test.ShouldBeNil)
		plannedPose := finalPoseInWorld.(*referenceframe.PoseInFrame).Pose()
		test.That(t, spatialmath.PoseAlmostEqualEps(plannedPose, finalPose.Pose(), .01), test.ShouldBeTrue)
	})

	t.Run("plan with custom start state", func(t *testing.T) {
		startConfig := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}
		startState := armplanning.NewPlanState(nil, referenceframe.FrameSystemInputs{
			"pieceArm": startConfig,
		})

		finalPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -180, Z: 34}))

		moveReq := motion.MoveReq{
			ComponentName: "pieceGripper",
			Destination:   finalPose,
			Extra: map[string]interface{}{
				"start_state": startState.Serialize(),
				"smooth_iter": 5,
			},
		}

		plan := getPlanFromMove(t, moveReq)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

		// Verify start configuration matches specified start state
		startArmConfig := plan[0]["pieceArm"]
		test.That(t, startArmConfig, test.ShouldResemble, startConfig)

		// Verify final pose
		frameSys, err := framesystem.NewFromService(ctx, ms.(*builtIn).fsService, nil)
		test.That(t, err, test.ShouldBeNil)

		finalConfig := plan[len(plan)-1]
		finalPoseInWorld, err := frameSys.Transform(finalConfig.ToLinearInputs(),
			referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewZeroPose()),
			"world")
		test.That(t, err, test.ShouldBeNil)
		plannedPose := finalPoseInWorld.(*referenceframe.PoseInFrame).Pose()
		test.That(t, spatialmath.PoseAlmostEqualEps(plannedPose, finalPose.Pose(), .01), test.ShouldBeTrue)
	})

	t.Run("plan with explicit goal state configuration", func(t *testing.T) {
		goalConfig := []float64{0.7, 0.6, 0.5, 0.4, 0.3, 0.2}

		goalState := armplanning.NewPlanState(nil, referenceframe.FrameSystemInputs{"pieceArm": goalConfig})

		moveReq := motion.MoveReq{
			ComponentName: "pieceGripper",
			Extra: map[string]interface{}{
				"goal_state":  goalState.Serialize(),
				"smooth_iter": 5,
			},
		}

		plan := getPlanFromMove(t, moveReq)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

		// Verify final configuration matches goal state
		finalArmConfig := plan[len(plan)-1]["pieceArm"]
		test.That(t, finalArmConfig, test.ShouldResemble, goalConfig)
	})
}

func TestConfiguredDefaultExtras(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("number of threads not configured", func(t *testing.T) {
		// test configuring the number of threads to be zero
		ms, err := NewBuiltIn(ctx, nil, resource.Config{ConvertedAttributes: &Config{}}, logger)
		test.That(t, err, test.ShouldBeNil)
		defer test.That(t, ms.Close(ctx), test.ShouldBeNil)

		// test that we can override the number of threads with user input
		extras := map[string]any{"num_threads": 1}
		ms.(*builtIn).applyDefaultExtras(extras)
		test.That(t, extras["num_threads"], test.ShouldEqual, 1)

		// test that if nothing is provided nothing is set
		extras = map[string]any{}
		ms.(*builtIn).applyDefaultExtras(extras)
		test.That(t, extras["num_threads"], test.ShouldBeNil)
	})

	t.Run("configure number of threads", func(t *testing.T) {
		// test configuring the number of threads to be a nonzero number
		ms, err := NewBuiltIn(ctx, nil, resource.Config{ConvertedAttributes: &Config{NumThreads: 10}}, logger)
		test.That(t, err, test.ShouldBeNil)
		defer test.That(t, ms.Close(ctx), test.ShouldBeNil)

		// test that we can override the number of threads with user input
		extras := map[string]any{"num_threads": 1}
		ms.(*builtIn).applyDefaultExtras(extras)
		test.That(t, extras["num_threads"], test.ShouldEqual, 1)

		// test that if nothing is provided we use the default
		extras = map[string]any{}
		ms.(*builtIn).applyDefaultExtras(extras)
		test.That(t, extras["num_threads"], test.ShouldEqual, 10)
	})

	t.Run("number of threads configured poorly", func(t *testing.T) {
		// test configuring the number of threads to be negative
		cfg := &Config{NumThreads: -1}
		_, _, err := cfg.Validate("")
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestConfigureJointLimits(t *testing.T) {
	ctx := context.Background()

	ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	defer teardown()

	svc := ms.(*builtIn)

	fs, err := svc.getFrameSystem(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	f := fs.Frame("pieceArm")
	test.That(t, f.DoF()[0].Min, test.ShouldAlmostEqual, -2*math.Pi)
	test.That(t, f.DoF()[1].Min, test.ShouldAlmostEqual, -2*math.Pi)

	svc.conf.InputRangeOverride = map[string]map[string]referenceframe.Limit{
		"pieceArm": {"0": referenceframe.Limit{0, 1}},
	}

	fs, err = svc.getFrameSystem(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	f = fs.Frame("pieceArm")
	test.That(t, f.DoF()[0].Min, test.ShouldAlmostEqual, 0)

	svc.conf.InputRangeOverride = map[string]map[string]referenceframe.Limit{}

	fs, err = svc.getFrameSystem(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	f = fs.Frame("pieceArm")
	test.That(t, f.DoF()[0].Min, test.ShouldAlmostEqual, -2*math.Pi)

	svc.conf.InputRangeOverride = map[string]map[string]referenceframe.Limit{
		"pieceArm": {"shoulder_lift_joint": referenceframe.Limit{0, 1}},
	}

	fs, err = svc.getFrameSystem(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	f = fs.Frame("pieceArm")
	test.That(t, f.DoF()[1].Min, test.ShouldAlmostEqual, 0)
}

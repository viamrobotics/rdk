package builtin

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/components/arm"
	armFake "go.viam.com/rdk/components/arm/fake"
	ur "go.viam.com/rdk/components/arm/universalrobots"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/state"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	viz "go.viam.com/rdk/vision"
)

func setupMotionServiceFromConfig(t *testing.T, configFilename string) (motion.Service, func()) {
	t.Helper()
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	svc, err := motion.FromRobot(myRobot, "builtin")
	test.That(t, err, test.ShouldBeNil)
	return svc, func() {
		myRobot.Close(context.Background())
	}
}

func TestMoveResponseString(t *testing.T) {
	type testCase struct {
		description  string
		expected     string
		moveResponse moveResponse
	}
	testCases := []testCase{
		{
			"when executeResponse.Replan is false & ReplanReason is empty and error is not nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: an error}",
			moveResponse{err: errors.New("an error")},
		},
		{
			"when executeResponse.Replan is true & ReplanReason is not empty and error is not nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: an error}",
			moveResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}, err: errors.New("an error")},
		},
		{
			"when executeResponse.Replan is true & ReplanReason is not empty and error is nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: <nil>}",
			moveResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}},
		},
		{
			"when executeResponse.Replan is false & ReplanReason is empty and error is nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: <nil>}",
			moveResponse{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			test.That(t, tc.moveResponse.String(), test.ShouldEqual, tc.expected)
		})
	}
}

func TestReplanResponseString(t *testing.T) {
	type testCase struct {
		description    string
		expected       string
		replanResponse replanResponse
	}
	testCases := []testCase{
		{
			"when replan is true and reason is non empty and error is nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: <nil>}",
			replanResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}},
		},
		{
			"when replan is true and reason is non empty and error is not nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: an error}",
			replanResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}, err: errors.New("an error")},
		},
		{
			"when replan is false and error is nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: <nil>}",
			replanResponse{},
		},
		{
			"when replan is false and error is not nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: an error}",
			replanResponse{err: errors.New("an error")},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			test.That(t, tc.replanResponse.String(), test.ShouldEqual, tc.expected)
		})
	}
}

func TestMoveFailures(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")
	defer teardown()
	ctx := context.Background()
	t.Run("fail on not finding gripper", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("fakeGripper", spatialmath.NewPoseFromPoint(r3.Vector{X: 10.0, Y: 10.0, Z: 10.0}))
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: gripper.Named("fake"), Destination: grabPose})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on nil destination", func(t *testing.T) {
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: arm.Named("arm1")})
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
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: arm.Named("arm1"), Destination: poseInFrame, WorldState: worldState})
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
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: gripper.Named("pieceGripper"), Destination: grabPose})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when mobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceArm", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: arm.Named("pieceArm"), Destination: grabPose})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when immobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, motion.MoveReq{ComponentName: gripper.Named("pieceGripper"), Destination: grabPose})
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
		moveReq := motion.MoveReq{ComponentName: gripper.Named("pieceGripper"), Destination: grabPose, WorldState: worldState}
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
			motion.MoveReq{ComponentName: gripper.Named("pieceArm"), Destination: grabPose, WorldState: worldState},
		)
		// This fails due to a large obstacle being in the way
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestPositionalReplanning(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()

	gpsPoint := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)
	epsilonMM := 150.
	pollingFreq := 10.
	motionCfg := &motion.MotionConfiguration{
		PositionPollingFreqHz: &pollingFreq,
		ObstaclePollingFreqHz: &defaultObstaclePollingHz,
		PlanDeviationMM:       epsilonMM,
		LinearMPerSec:         0.2,
		AngularDegsPerSec:     20,
	}

	type testCase struct {
		name            string
		noise           r3.Vector
		expectedSuccess bool
		expectedErr     string
		extra           map[string]interface{}
	}

	testCases := []testCase{
		{
			name:            "check we dont replan with a good sensor",
			noise:           r3.Vector{},
			expectedSuccess: true,
			extra:           map[string]interface{}{"max_replans": 0, "smooth_iter": 5},
		},
		// TODO(RSDK-5634): this should be uncommented when this bug is fixed
		// {
		// 	// This also checks that `replan` is called under default conditions when "max_replans" is not set
		// 	name:            "check we fail to replan with a low cost factor",
		// 	noise:           r3.Vector{Y: epsilonMM + 0.1},
		// 	expectedErr:     "unable to create a new plan within replanCostFactor from the original",
		// 	expectedSuccess: false,
		// 	extra:           map[string]interface{}{"replan_cost_factor": 0.01, "smooth_iter": 5},
		// },
		{
			name:            "check we replan with a noisy sensor",
			noise:           r3.Vector{Y: epsilonMM + 0.1},
			expectedErr:     fmt.Sprintf("exceeded maximum number of replans: %d: plan failed", 4),
			expectedSuccess: false,
			extra:           map[string]interface{}{"replan_cost_factor": 10.0, "max_replans": 4, "smooth_iter": 5},
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, gpsPoint, 80, spatialmath.NewPoseFromPoint(tc.noise))
		defer closeFunc(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      resource.NewName(base.API, baseName),
			Destination:        dst,
			MovementSensorName: resource.NewName(movementsensor.API, moveSensorName),
			MotionCfg:          motionCfg,
			Extra:              tc.extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Minute*5)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})

		if tc.expectedSuccess {
			test.That(t, err, test.ShouldBeNil)
		} else {
			test.That(t, err, test.ShouldNotBeNil) // Needed so that a failure produces a failure and not a panic
			test.That(t, err.Error(), test.ShouldEqual, tc.expectedErr)
		}
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			testFn(t, c)
		})
	}
}

func TestObstacleReplanningSlam(t *testing.T) {
	cameraPoseInBase := spatialmath.NewPose(r3.Vector{0, 0, 0}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90})

	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	origin := spatialmath.NewPose(
		r3.Vector{X: -900, Y: 0, Z: 0},
		&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -90},
	)

	boxWrld, err := spatialmath.NewBox(
		spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
		r3.Vector{X: 50, Y: 50, Z: 50}, "box-obstacle",
	)
	test.That(t, err, test.ShouldBeNil)

	kb, ms, closeFunc := CreateMoveOnMapTestEnvironment(
		ctx, t,
		"pointcloud/cardboardOcto.pcd",
		50, origin,
	)
	defer closeFunc(ctx)

	// This vision service should return nothing the first time it is called, and should return an obstacle all other times.
	// In this way we generate a valid plan, and then can create a transient obstacle which we must route around.
	visSrvc, ok := ms.(*builtIn).visionServices[vision.Named("test-vision")].(*inject.VisionService)
	test.That(t, ok, test.ShouldBeTrue)
	i := 0
	visSrvc.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
		if i == 0 {
			i++
			return []*viz.Object{}, nil
		}
		currentPif, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		relativeBox := boxWrld.Transform(spatialmath.PoseBetween(spatialmath.Compose(currentPif.Pose(), cameraPoseInBase), boxWrld.Pose()))
		detection, err := viz.NewObjectWithLabel(pointcloud.New(), "test-case-1-detection", relativeBox.ToProtobuf())
		test.That(t, err, test.ShouldBeNil)

		return []*viz.Object{detection}, nil
	}

	obstacleDetectorSlice := []motion.ObstacleDetectorName{
		{VisionServiceName: vision.Named("test-vision"), CameraName: camera.Named("test-camera")},
	}
	positionPollingFreq := 0.
	obstaclePollingFreq := 5.
	req := motion.MoveOnMapReq{
		ComponentName: base.Named("test-base"),
		Destination:   spatialmath.NewPoseFromPoint(r3.Vector{X: 800, Y: 0, Z: 0}),
		SlamName:      slam.Named("test_slam"),
		MotionCfg: &motion.MotionConfiguration{
			PositionPollingFreqHz: &positionPollingFreq,
			ObstaclePollingFreqHz: &obstaclePollingFreq,
			PlanDeviationMM:       1000,
			ObstacleDetectors:     obstacleDetectorSlice,
		},
		Extra: map[string]interface{}{"smooth_iter": 20},
	}

	executionID, err := ms.MoveOnMap(ctx, req)
	test.That(t, err, test.ShouldBeNil)

	timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*45)
	defer timeoutFn()
	err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond, motion.PlanHistoryReq{
		ComponentName: req.ComponentName,
		ExecutionID:   executionID,
		LastPlanOnly:  true,
	})
	test.That(t, err, test.ShouldBeNil)
}

func TestMultiplePieces(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/fake_tomato.json")
	defer teardown()
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: -0, Y: -30, Z: -50}))
	_, err = ms.Move(context.Background(), motion.MoveReq{ComponentName: gripper.Named("gr"), Destination: grabPose})
	test.That(t, err, test.ShouldBeNil)
}

func TestGetPose(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")
	defer teardown()

	pose, err := ms.GetPose(context.Background(), arm.Named("gantry1"), "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 1.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 500)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("gantry1"), "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "arm1", nil, map[string]interface{}{})
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

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame2", transforms, map[string]interface{}{})
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
	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame", transforms, map[string]interface{}{})
	test.That(t, err, test.ShouldBeError, referenceframe.NewParentFrameMissingError("testFrame", "noParent"))
	test.That(t, pose, test.ShouldBeNil)
}

func TestStoppableMoveFunctions(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	failToReachGoalError := errors.New("failed to reach goal")
	calledStopFunc := false
	testIfStoppable := func(t *testing.T, success bool, err, expectedErr error) {
		t.Helper()
		test.That(t, err, test.ShouldBeError, expectedErr)
		test.That(t, success, test.ShouldBeFalse)
		test.That(t, calledStopFunc, test.ShouldBeTrue)
	}
	extra := map[string]interface{}{"smooth_iter": 5}

	t.Run("successfully stop arms", func(t *testing.T) {
		armName := "test-arm"
		injectArmName := arm.Named(armName)
		goal := referenceframe.NewPoseInFrame(
			armName,
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -10, Z: -10}),
		)

		// Create an injected Arm
		armCfg := resource.Config{
			Name:  armName,
			API:   arm.API,
			Model: resource.DefaultModelFamily.WithModel("ur5e"),
			ConvertedAttributes: &armFake.Config{
				ArmModel: "ur5e",
			},
			Frame: &referenceframe.LinkConfig{
				Parent: "world",
			},
		}

		fakeArm, err := armFake.NewArm(ctx, nil, armCfg, logger)
		test.That(t, err, test.ShouldBeNil)

		injectArm := &inject.Arm{
			Arm: fakeArm,
		}
		injectArm.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			calledStopFunc = true
			return nil
		}
		injectArm.GoToInputsFunc = func(ctx context.Context, goal ...[]referenceframe.Input) error {
			return failToReachGoalError
		}
		injectArm.ModelFrameFunc = func() referenceframe.Model {
			model, _ := ur.MakeModelFrame("ur5e")
			return model
		}
		injectArm.MoveToPositionFunc = func(ctx context.Context, to spatialmath.Pose, extra map[string]interface{}) error {
			return failToReachGoalError
		}

		// create arm link
		armLink := referenceframe.NewLinkInFrame(
			referenceframe.World,
			spatialmath.NewZeroPose(),
			armName,
			nil,
		)

		// Create a motion service
		fsParts := []*referenceframe.FrameSystemPart{
			{
				FrameConfig: armLink,
				ModelFrame:  injectArm.ModelFrameFunc(),
			},
		}
		deps := resource.Dependencies{
			injectArmName: injectArm,
		}

		_, err = createFrameSystemService(ctx, deps, fsParts, logger)
		test.That(t, err, test.ShouldBeNil)

		conf := resource.Config{ConvertedAttributes: &Config{}}
		ms, err := NewBuiltIn(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldBeNil)
		defer ms.Close(context.Background())

		t.Run("stop during Move(...) call", func(t *testing.T) {
			calledStopFunc = false
			success, err := ms.Move(ctx, motion.MoveReq{ComponentName: injectArmName, Destination: goal, Extra: extra})
			testIfStoppable(t, success, err, failToReachGoalError)
		})
	})

	t.Run("successfully stop kinematic bases", func(t *testing.T) {
		// Create an injected Base
		baseName := "test-base"

		geometry, err := (&spatialmath.GeometryConfig{R: 20}).ParseConfig()
		test.That(t, err, test.ShouldBeNil)

		injectBase := inject.NewBase(baseName)
		injectBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			return []spatialmath.Geometry{geometry}, nil
		}
		injectBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
			return base.Properties{
				TurningRadiusMeters: 0,
				WidthMeters:         600 * 0.001,
			}, nil
		}
		injectBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			calledStopFunc = true
			return nil
		}
		injectBase.SpinFunc = func(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
			return failToReachGoalError
		}
		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
			return failToReachGoalError
		}
		injectBase.SetVelocityFunc = func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
			return failToReachGoalError
		}

		// Create a base link
		baseLink := createBaseLink(t)

		t.Run("stop during MoveOnGlobe(...) call", func(t *testing.T) {
			calledStopFunc = false
			gpsPoint := geo.NewPoint(-70, 40)

			// Create an injected MovementSensor
			movementSensorName := "test-gps"
			injectMovementSensor := createInjectedMovementSensor(movementSensorName, gpsPoint)

			// Create a MovementSensor link
			movementSensorLink := referenceframe.NewLinkInFrame(
				baseLink.Name(),
				spatialmath.NewPoseFromPoint(r3.Vector{X: -10, Y: 0, Z: 0}),
				movementSensorName,
				nil,
			)

			// Create a motion service
			fsParts := []*referenceframe.FrameSystemPart{
				{FrameConfig: movementSensorLink},
				{FrameConfig: baseLink},
			}
			deps := resource.Dependencies{
				injectBase.Name():           injectBase,
				injectMovementSensor.Name(): injectMovementSensor,
			}

			fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
			test.That(t, err, test.ShouldBeNil)

			conf := resource.Config{ConvertedAttributes: &Config{}}
			ms, err := NewBuiltIn(ctx, deps, conf, logger)
			test.That(t, err, test.ShouldBeNil)
			defer ms.Close(context.Background())

			ms.(*builtIn).fsService = fsSvc

			goal := geo.NewPoint(gpsPoint.Lat()+1e-4, gpsPoint.Lng()+1e-4)
			motionCfg := motion.MotionConfiguration{
				PlanDeviationMM: 10000,
				LinearMPerSec:   10,
			}

			req := motion.MoveOnGlobeReq{
				ComponentName:      injectBase.Name(),
				Destination:        goal,
				MovementSensorName: injectMovementSensor.Name(),
				MotionCfg:          &motionCfg,
				Extra:              extra,
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeNil)

			timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
				ComponentName: req.ComponentName,
				ExecutionID:   executionID,
				LastPlanOnly:  true,
			})

			expectedErr := errors.Wrap(errors.New("plan failed"), failToReachGoalError.Error())
			testIfStoppable(t, false, err, expectedErr)
		})

		t.Run("stop during MoveOnMap(...) call", func(t *testing.T) {
			calledStopFunc = false
			slamName := "test-slam"

			// Create an injected SLAM
			injectSlam := createInjectedSlam(slamName)

			// Create a motion service
			deps := resource.Dependencies{
				injectBase.Name(): injectBase,
				injectSlam.Name(): injectSlam,
			}
			fsParts := []*referenceframe.FrameSystemPart{
				{FrameConfig: baseLink},
			}

			ms, err := NewBuiltIn(
				ctx,
				deps,
				resource.Config{ConvertedAttributes: &Config{}},
				logger,
			)
			test.That(t, err, test.ShouldBeNil)
			defer ms.Close(context.Background())

			fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
			test.That(t, err, test.ShouldBeNil)
			ms.(*builtIn).fsService = fsSvc

			goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 500})
			req := motion.MoveOnMapReq{
				ComponentName: injectBase.Name(),
				Destination:   goal,
				SlamName:      injectSlam.Name(),
				MotionCfg: &motion.MotionConfiguration{
					PlanDeviationMM: 0.2,
				},
				Extra: extra,
			}

			executionID, err := ms.MoveOnMap(ctx, req)
			test.That(t, err, test.ShouldBeNil)

			timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
				ComponentName: req.ComponentName,
				ExecutionID:   executionID,
				LastPlanOnly:  true,
			})

			expectedErr := errors.Wrap(errors.New("plan failed"), failToReachGoalError.Error())
			testIfStoppable(t, false, err, expectedErr)
		})

		t.Run("stop during MoveOnMap(...) call", func(t *testing.T) {
			calledStopFunc = false
			slamName := "test-slam"

			// Create an injected SLAM
			injectSlam := createInjectedSlam(slamName)

			// Create a motion service
			deps := resource.Dependencies{
				injectBase.Name(): injectBase,
				injectSlam.Name(): injectSlam,
			}
			fsParts := []*referenceframe.FrameSystemPart{
				{FrameConfig: baseLink},
			}

			ms, err := NewBuiltIn(
				ctx,
				deps,
				resource.Config{ConvertedAttributes: &Config{}},
				logger,
			)
			test.That(t, err, test.ShouldBeNil)
			defer ms.Close(context.Background())

			fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
			test.That(t, err, test.ShouldBeNil)
			ms.(*builtIn).fsService = fsSvc

			req := motion.MoveOnMapReq{
				ComponentName: injectBase.Name(),
				Destination:   spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 500}),
				SlamName:      injectSlam.Name(),
				MotionCfg: &motion.MotionConfiguration{
					PlanDeviationMM: 1,
				},
				Extra: extra,
			}

			executionID, err := ms.MoveOnMap(ctx, req)
			test.That(t, err, test.ShouldBeNil)

			timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
				ComponentName: req.ComponentName,
				ExecutionID:   executionID,
				LastPlanOnly:  true,
			})

			expectedErr := errors.Wrap(errors.New("plan failed"), failToReachGoalError.Error())
			testIfStoppable(t, false, err, expectedErr)
		})
	})
}

func TestGetTransientDetectionsSlam(t *testing.T) {
	ctx := context.Background()

	_, ms, closeFunc := CreateMoveOnMapTestEnvironment(
		ctx, t,
		"slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd",
		100, spatialmath.NewZeroPose(),
	)
	t.Cleanup(func() { closeFunc(ctx) })

	// construct move request
	moveReq := motion.MoveOnMapReq{
		ComponentName: base.Named("test-base"),
		Destination:   spatialmath.NewPoseFromPoint(r3.Vector{X: 10, Y: 0, Z: 0}),
		SlamName:      slam.Named("test_slam"),
		MotionCfg: &motion.MotionConfiguration{
			PlanDeviationMM: 1,
			ObstacleDetectors: []motion.ObstacleDetectorName{
				{VisionServiceName: vision.Named("test-vision"), CameraName: camera.Named("test-camera")},
			},
		},
	}

	planExecutor, err := ms.(*builtIn).newMoveOnMapRequest(ctx, moveReq, nil, 0)
	test.That(t, err, test.ShouldBeNil)

	mr, ok := planExecutor.(*moveRequest)
	test.That(t, ok, test.ShouldBeTrue)

	injectedVis, ok := ms.(*builtIn).visionServices[vision.Named("test-vision")].(*inject.VisionService)
	test.That(t, ok, test.ShouldBeTrue)

	// define injected method on vision service
	injectedVis.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
		boxGeom, err := spatialmath.NewBox(
			spatialmath.NewPose(r3.Vector{4, 8, 10}, &spatialmath.OrientationVectorDegrees{OZ: 1}),
			r3.Vector{2, 3, 5},
			"test-box",
		)
		test.That(t, err, test.ShouldBeNil)
		detection, err := viz.NewObjectWithLabel(pointcloud.New(), "test-box", boxGeom.ToProtobuf())
		test.That(t, err, test.ShouldBeNil)
		return []*viz.Object{detection}, nil
	}

	type testCase struct {
		name          string
		detectionPose spatialmath.Pose
	}
	testCases := []testCase{
		{
			name:          "relative - SLAM/base theta does not matter",
			detectionPose: spatialmath.NewPose(r3.Vector{4, 10, -8}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90}),
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		transformedGeoms, err := mr.getTransientDetections(ctx, injectedVis, camera.Named("test-camera"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, transformedGeoms.Parent(), test.ShouldEqual, referenceframe.World)
		test.That(t, len(transformedGeoms.Geometries()), test.ShouldEqual, 1)
		test.That(t, spatialmath.PoseAlmostEqual(transformedGeoms.Geometries()[0].Pose(), tc.detectionPose), test.ShouldBeTrue)
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			testFn(t, c)
		})
	}
}

func TestGetTransientDetectionsMath(t *testing.T) {
	ctx := context.Background()

	_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(
		ctx, t, geo.NewPoint(0, 0), 300, spatialmath.NewZeroPose(),
	)
	t.Cleanup(func() { closeFunc(ctx) })

	destinationPose := spatialmath.NewPoseFromPoint(r3.Vector{0, 3000, 0})
	geoPoseOrigin := spatialmath.NewGeoPose(geo.NewPoint(0, 0), 0)
	destinationGeoPose := spatialmath.PoseToGeoPose(geoPoseOrigin, destinationPose)

	moveSensor, ok := ms.(*builtIn).movementSensors[resource.NewName(movementsensor.API, moveSensorName)]
	test.That(t, ok, test.ShouldBeTrue)

	// construct move request
	moveReq := motion.MoveOnGlobeReq{
		ComponentName:      base.Named("test-base"),
		Destination:        destinationGeoPose.Location(),
		MovementSensorName: moveSensor.Name(),
		MotionCfg: &motion.MotionConfiguration{
			PlanDeviationMM: 1,
			ObstacleDetectors: []motion.ObstacleDetectorName{
				{VisionServiceName: vision.Named("injectedVisionSvc"), CameraName: camera.Named("test-camera")},
			},
		},
	}

	planExecutor, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, moveReq, nil, 0)
	test.That(t, err, test.ShouldBeNil)

	mr, ok := planExecutor.(*moveRequest)
	test.That(t, ok, test.ShouldBeTrue)

	getTransientDetectionMock := func(currentPose, obstaclePose spatialmath.Pose) []spatialmath.Geometry {
		inputMap, _, err := mr.fsService.CurrentInputs(ctx)
		test.That(t, err, test.ShouldBeNil)
		kbInputs := make([]referenceframe.Input, len(mr.kinematicBase.Kinematics().DoF()))
		kbInputs = append(kbInputs, referenceframe.PoseToInputs(currentPose)...)
		inputMap[mr.kinematicBase.Name().ShortName()] = kbInputs

		fakeDetectionGeometry, err := spatialmath.NewBox(
			obstaclePose, r3.Vector{X: 8, Y: 4, Z: 2}, "box",
		)
		test.That(t, err, test.ShouldBeNil)

		cam, ok := ms.(*builtIn).components[resource.NewName(camera.API, "injectedCamera")]
		test.That(t, ok, test.ShouldBeTrue)

		tf, err := mr.localizingFS.Transform(
			inputMap,
			referenceframe.NewGeometriesInFrame(
				cam.Name().ShortName(), []spatialmath.Geometry{fakeDetectionGeometry},
			),
			referenceframe.World,
		)
		test.That(t, err, test.ShouldBeNil)

		worldGifs, ok := tf.(*referenceframe.GeometriesInFrame)
		test.That(t, ok, test.ShouldBeTrue)
		return worldGifs.Geometries()
	}

	type testCase struct {
		name                                     string
		basePose, obstaclePose, expectedGeomPose spatialmath.Pose
	}

	testCases := []testCase{
		{
			name:             "base at zero pose",
			basePose:         spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1}),
			obstaclePose:     spatialmath.NewPoseFromPoint(r3.Vector{X: 10, Y: -30, Z: 1500}),
			expectedGeomPose: spatialmath.NewPose(r3.Vector{X: 10, Y: 1500, Z: 30}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90}),
		},
		{
			name:             "base slightly from origin with zero theta",
			basePose:         spatialmath.NewPose(r3.Vector{X: 3, Y: 9, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1}),
			obstaclePose:     spatialmath.NewPoseFromPoint(r3.Vector{X: 10, Y: -30, Z: 1500}),
			expectedGeomPose: spatialmath.NewPose(r3.Vector{X: 13, Y: 1509, Z: 30}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90}),
		},
		{
			name:         "base at origin with non-zero theta",
			basePose:     spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -45}),
			obstaclePose: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: math.Sqrt2}),
			expectedGeomPose: spatialmath.NewPose(
				r3.Vector{X: 1, Y: 1, Z: 0}, &spatialmath.OrientationVectorDegrees{OX: 0.7071067811865478, OY: 0.7071067811865474, Theta: -90},
			),
		},
		{
			name:         "base slightly from origin with non-zero theta",
			basePose:     spatialmath.NewPose(r3.Vector{X: 3, Y: 9, Z: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -45}),
			obstaclePose: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: math.Sqrt2}),
			expectedGeomPose: spatialmath.NewPose(
				r3.Vector{X: 4, Y: 10, Z: 0}, &spatialmath.OrientationVectorDegrees{OX: 0.7071067811865478, OY: 0.7071067811865474, Theta: -90},
			),
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		transformedGeoms := getTransientDetectionMock(tc.basePose, tc.obstaclePose)
		test.That(t, len(transformedGeoms), test.ShouldEqual, 1)

		test.That(t, spatialmath.PoseAlmostEqual(transformedGeoms[0].Pose(), tc.expectedGeomPose), test.ShouldBeTrue)
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			testFn(t, c)
		})
	}
}

func TestStopPlan(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	gpsPoint := geo.NewPoint(0, 0)

	_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, gpsPoint, 80, nil)
	defer closeFunc(ctx)

	req := motion.StopPlanReq{}
	err := ms.StopPlan(ctx, req)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(req.ComponentName))
}

func TestListPlanStatuses(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	gpsPoint := geo.NewPoint(0, 0)

	_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, gpsPoint, 80, nil)
	defer closeFunc(ctx)

	req := motion.ListPlanStatusesReq{}
	// returns no results as no move on globe calls have been made
	planStatusesWithIDs, err := ms.ListPlanStatuses(ctx, req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planStatusesWithIDs), test.ShouldEqual, 0)
}

func TestPlanHistory(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	gpsPoint := geo.NewPoint(0, 0)

	_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, gpsPoint, 80, nil)
	defer closeFunc(ctx)
	req := motion.PlanHistoryReq{}
	history, err := ms.PlanHistory(ctx, req)
	test.That(t, err, test.ShouldResemble, resource.NewNotFoundError(req.ComponentName))
	test.That(t, history, test.ShouldBeNil)
}

func TestBaseInputs(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	kb, closeFunc := createTestKinematicBase(
		ctx,
		t,
	)
	defer closeFunc(ctx)
	err := kb.GoToInputs(ctx, []referenceframe.Input{{0}, {0.001 + math.Pi/2}, {0}, {91}})
	test.That(t, err, test.ShouldBeNil)
}

func TestCheckPlan(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	origin := geo.NewPoint(0, 0)

	localizer, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, origin, 30, spatialmath.NewZeroPose())
	defer closeFunc(ctx)

	dst := geo.NewPoint(origin.Lat(), origin.Lng()+5e-5)
	// Note: spatialmath.GeoPointToPoint(dst, origin) produces r3.Vector{5559.746, 0, 0}

	movementSensor, ok := localizer.(movementsensor.MovementSensor)
	test.That(t, ok, test.ShouldBeTrue)

	fakeBase, ok := ms.(*builtIn).components[baseResource]
	test.That(t, ok, test.ShouldBeTrue)

	req := motion.MoveOnGlobeReq{
		ComponentName:      fakeBase.Name(),
		Destination:        dst,
		MovementSensorName: movementSensor.Name(),
	}

	// construct move request
	planExecutor, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, req, nil, 0)
	test.That(t, err, test.ShouldBeNil)
	mr, ok := planExecutor.(*moveRequest)
	test.That(t, ok, test.ShouldBeTrue)

	// construct plan
	plan, err := mr.Plan(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(plan.Path()), test.ShouldBeGreaterThan, 2)

	wrapperFrame := mr.localizingFS.Frame(mr.kinematicBase.Name().Name)

	currentInputs := referenceframe.FrameSystemInputs{
		mr.kinematicBase.Kinematics().Name(): {
			{Value: 0}, // ptg index
			{Value: 0}, // trajectory alpha within ptg
			{Value: 0}, // start distance along trajectory index
			{Value: 0}, // end distace along trajectory index
		},
		mr.kinematicBase.LocalizationFrame().Name(): {
			{Value: 0}, // X
			{Value: 0}, // Y
			{Value: 0}, // Z
			{Value: 0}, // OX
			{Value: 0}, // OY
			{Value: 1}, // OZ
			{Value: 0}, // Theta
		},
	}

	baseExecutionState, err := motionplan.NewExecutionState(
		plan, 0, currentInputs,
		map[string]*referenceframe.PoseInFrame{
			mr.kinematicBase.LocalizationFrame().Name(): referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPose(
				r3.Vector{X: 0, Y: 0, Z: 0},
				&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0},
			)),
		},
	)
	test.That(t, err, test.ShouldBeNil)

	augmentedBaseExecutionState, err := mr.augmentBaseExecutionState(baseExecutionState)
	test.That(t, err, test.ShouldBeNil)

	t.Run("base case - validate plan without obstacles", func(t *testing.T) {
		err = motionplan.CheckPlan(wrapperFrame, augmentedBaseExecutionState, nil, mr.localizingFS, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("obstacles blocking path", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2000, 0, 0}), r3.Vector{20, 2000, 1}, "")
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(wrapperFrame, augmentedBaseExecutionState, worldState, mr.localizingFS, math.Inf(1), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, strings.Contains(err.Error(), "found constraint violation or collision in segment between"), test.ShouldBeTrue)
	})

	// create camera_origin frame
	cameraOriginFrame, err := referenceframe.NewStaticFrame("camera-origin", spatialmath.NewPoseFromPoint(r3.Vector{0, -20, 0}))
	test.That(t, err, test.ShouldBeNil)
	err = mr.localizingFS.AddFrame(cameraOriginFrame, wrapperFrame)
	test.That(t, err, test.ShouldBeNil)

	// create camera geometry
	cameraGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{1, 1, 1}, "camera")
	test.That(t, err, test.ShouldBeNil)

	// create cameraFrame and add to framesystem
	cameraFrame, err := referenceframe.NewStaticFrameWithGeometry(
		"camera-frame", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}), cameraGeom,
	)
	test.That(t, err, test.ShouldBeNil)
	err = mr.localizingFS.AddFrame(cameraFrame, cameraOriginFrame)
	test.That(t, err, test.ShouldBeNil)
	inputs := augmentedBaseExecutionState.CurrentInputs()
	inputs[cameraFrame.Name()] = referenceframe.FloatsToInputs(make([]float64, len(cameraFrame.DoF())))
	executionStateWithCamera, err := motionplan.NewExecutionState(
		augmentedBaseExecutionState.Plan(), augmentedBaseExecutionState.Index(),
		inputs, augmentedBaseExecutionState.CurrentPoses(),
	)
	test.That(t, err, test.ShouldBeNil)

	t.Run("obstacles NOT in world frame - no collision - integration test", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{25000, -40, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(wrapperFrame, executionStateWithCamera, worldState, mr.localizingFS, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("obstacles NOT in world frame cause collision - integration test", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{2500, 20, 0}),
			r3.Vector{10, 2000, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(wrapperFrame, executionStateWithCamera, worldState, mr.localizingFS, math.Inf(1), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, strings.Contains(err.Error(), "found constraint violation or collision in segment between"), test.ShouldBeTrue)
	})

	currentInputs = referenceframe.FrameSystemInputs{
		mr.kinematicBase.Kinematics().Name(): {
			{Value: 0}, // ptg index
			{Value: 0}, // trajectory alpha within ptg
			{Value: 0}, // start distance along trajectory index
			{Value: 0}, // end distace along trajectory index
		},
		mr.kinematicBase.LocalizationFrame().Name(): {
			{Value: 2779.937}, // X
			{Value: 0},        // Y
			{Value: 0},        // Z
			{Value: 0},        // OX
			{Value: 0},        // OY
			{Value: 1},        // OZ
			{Value: -math.Pi}, // Theta
		},
		cameraFrame.Name(): referenceframe.FloatsToInputs(make([]float64, len(cameraFrame.DoF()))),
	}
	currentPoses := map[string]*referenceframe.PoseInFrame{
		mr.kinematicBase.LocalizationFrame().Name(): referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPose(
			r3.Vector{X: 2779.937, Y: 0, Z: 0},
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -90},
		)),
	}

	newExecutionState, err := motionplan.NewExecutionState(plan, 2, currentInputs, currentPoses)
	test.That(t, err, test.ShouldBeNil)
	updatedExecutionState, err := mr.augmentBaseExecutionState(newExecutionState)
	test.That(t, err, test.ShouldBeNil)

	t.Run("checking from partial-plan, ensure success with obstacles - integration test", func(t *testing.T) {
		// create obstacle behind where we are
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{0, 20, 0}),
			r3.Vector{10, 200, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(wrapperFrame, updatedExecutionState, worldState, mr.localizingFS, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("verify partial plan with non-nil errorState and obstacle", func(t *testing.T) {
		// create obstacle which is behind where the robot already is, but is on the path it has already traveled
		box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{10, 10, 1}, "obstacle")
		test.That(t, err, test.ShouldBeNil)
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{box})}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(wrapperFrame, updatedExecutionState, worldState, mr.localizingFS, math.Inf(1), logger)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestDoCommand(t *testing.T) {
	ctx := context.Background()
	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{1000, 1000, 1000}), r3.Vector{1, 1, 1}, "box")
	test.That(t, err, test.ShouldBeNil)
	geometries := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame("world", []spatialmath.Geometry{box})}
	worldState, err := referenceframe.NewWorldState(geometries, nil)
	test.That(t, err, test.ShouldBeNil)
	moveReq := motion.MoveReq{
		ComponentName: gripper.Named("pieceGripper"),
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
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()

		plan, err := ms.(*builtIn).plan(ctx, moveReq)
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

	t.Run("Extras transmitted correctly", func(t *testing.T) {
		// test that DoPlan correctly breaks if bad inputs are provided, meaning it is being parsed correctly
		moveReq.Extra = map[string]interface{}{
			"motion_profile": motionplan.LinearMotionProfile,
			"planning_alg":   "rrtstar",
		}
		_, err = testDoPlan(moveReq)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldResemble, motionplan.NewAlgAndConstraintMismatchErr("rrtstar"))
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

		wp1State := motionplan.NewPlanState(referenceframe.FrameSystemPoses{"pieceGripper": waypoint1}, nil)
		wp2State := motionplan.NewPlanState(referenceframe.FrameSystemPoses{"pieceGripper": waypoint2}, nil)

		moveReq := motion.MoveReq{
			ComponentName: gripper.Named("pieceGripper"),
			Destination:   finalPose,
			Extra: map[string]interface{}{
				"waypoints":   []interface{}{wp1State.Serialize(), wp2State.Serialize()},
				"smooth_iter": 5,
			},
		}

		plan := getPlanFromMove(t, moveReq)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

		// Verify start configuration matches current robot state
		fsInputs, _, err := ms.(*builtIn).fsService.CurrentInputs(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, plan[0], test.ShouldResemble, fsInputs)

		// Verify final pose
		frameSys, err := ms.(*builtIn).fsService.FrameSystem(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		finalConfig := plan[len(plan)-1]
		finalPoseInWorld, err := frameSys.Transform(finalConfig,
			referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewZeroPose()),
			"world")
		test.That(t, err, test.ShouldBeNil)
		plannedPose := finalPoseInWorld.(*referenceframe.PoseInFrame).Pose()
		test.That(t, spatialmath.PoseAlmostEqualEps(plannedPose, finalPose.Pose(), 1e-3), test.ShouldBeTrue)
	})

	t.Run("plan through mixed pose and configuration waypoints", func(t *testing.T) {
		// Define specific arm configuration for first waypoint
		armConfig := []float64{0.2, 0.3, 0.4, 0.5, 0.6, 0.7}
		wp1State := motionplan.NewPlanState(nil, referenceframe.FrameSystemInputs{
			"pieceArm": referenceframe.FloatsToInputs(armConfig),
		})

		// Define pose for second waypoint
		intermediatePose := spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -190, Z: 30})
		wp2State := motionplan.NewPlanState(
			referenceframe.FrameSystemPoses{"pieceGripper": referenceframe.NewPoseInFrame("world", intermediatePose)},
			nil,
		)

		finalPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -180, Z: 34}))

		moveReq := motion.MoveReq{
			ComponentName: gripper.Named("pieceGripper"),
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
					if math.Abs(val.Value-armConfig[i]) > 1e-3 {
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
		frameSys, err := ms.(*builtIn).fsService.FrameSystem(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		finalConfig := plan[len(plan)-1]
		finalPoseInWorld, err := frameSys.Transform(finalConfig,
			referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewZeroPose()),
			"world")
		test.That(t, err, test.ShouldBeNil)
		plannedPose := finalPoseInWorld.(*referenceframe.PoseInFrame).Pose()
		test.That(t, spatialmath.PoseAlmostEqualEps(plannedPose, finalPose.Pose(), 1e-3), test.ShouldBeTrue)
	})

	t.Run("plan with custom start state", func(t *testing.T) {
		startConfig := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}
		startState := motionplan.NewPlanState(nil, referenceframe.FrameSystemInputs{
			"pieceArm": referenceframe.FloatsToInputs(startConfig),
		})

		finalPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -800, Y: -180, Z: 34}))

		moveReq := motion.MoveReq{
			ComponentName: gripper.Named("pieceGripper"),
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
		test.That(t, startArmConfig, test.ShouldResemble, referenceframe.FloatsToInputs(startConfig))

		// Verify final pose
		frameSys, err := ms.(*builtIn).fsService.FrameSystem(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		finalConfig := plan[len(plan)-1]
		finalPoseInWorld, err := frameSys.Transform(finalConfig,
			referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewZeroPose()),
			"world")
		test.That(t, err, test.ShouldBeNil)
		plannedPose := finalPoseInWorld.(*referenceframe.PoseInFrame).Pose()
		test.That(t, spatialmath.PoseAlmostEqualEps(plannedPose, finalPose.Pose(), 1e-3), test.ShouldBeTrue)
	})

	t.Run("plan with explicit goal state configuration", func(t *testing.T) {
		goalConfig := []float64{0.7, 0.6, 0.5, 0.4, 0.3, 0.2}

		goalState := motionplan.NewPlanState(nil, referenceframe.FrameSystemInputs{"pieceArm": referenceframe.FloatsToInputs(goalConfig)})

		moveReq := motion.MoveReq{
			ComponentName: gripper.Named("pieceGripper"),
			Extra: map[string]interface{}{
				"goal_state":  goalState.Serialize(),
				"smooth_iter": 5,
			},
		}

		plan := getPlanFromMove(t, moveReq)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

		// Verify final configuration matches goal state
		finalArmConfig := plan[len(plan)-1]["pieceArm"]
		test.That(t, finalArmConfig, test.ShouldResemble, referenceframe.FloatsToInputs(goalConfig))
	})
}

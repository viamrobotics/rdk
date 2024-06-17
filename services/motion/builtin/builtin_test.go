package builtin

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	armFake "go.viam.com/rdk/components/arm/fake"
	ur "go.viam.com/rdk/components/arm/universalrobots"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/state"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	viz "go.viam.com/rdk/vision"
)

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
		grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{X: 10.0, Y: 10.0, Z: 10.0}))
		_, err = ms.Move(ctx, camera.Named("fake"), grabPose, nil, nil, nil)
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
		_, err = ms.Move(ctx, arm.Named("arm1"), poseInFrame, worldState, nil, nil)
		test.That(t, err, test.ShouldBeError, referenceframe.NewParentFrameMissingError("frame2", "noParent"))
	})
}

func TestMove(t *testing.T) {
	var err error
	ctx := context.Background()

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, gripper.Named("pieceGripper"), grabPose, nil, nil, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when mobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceArm", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, arm.Named("pieceArm"), grabPose, nil, nil, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when immobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, gripper.Named("pieceGripper"), grabPose, nil, nil, map[string]interface{}{})
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
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, worldState, nil, nil)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMoveWithObstacles(t *testing.T) {
	ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	defer teardown()

	t.Run("check a movement that should not succeed due to obstacles", func(t *testing.T) {
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
		_, err = ms.Move(context.Background(), gripper.Named("pieceArm"), grabPose, worldState, nil, nil)
		// This fails due to a large obstacle being in the way
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestPositionalReplanning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()

	gpsPoint := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)
	epsilonMM := 15.
	motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 100, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}

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
			noise:           r3.Vector{Y: epsilonMM - 0.1},
			expectedSuccess: true,
			extra:           map[string]interface{}{"smooth_iter": 5},
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
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, spatialmath.NewPoseFromPoint(tc.noise), 5)
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

func TestObstacleReplanningGlobe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()

	gpsOrigin := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsOrigin.Lat(), gpsOrigin.Lng()+1e-5)
	epsilonMM := 15.

	type testCase struct {
		name            string
		getPCfunc       func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error)
		expectedSuccess bool
		expectedErr     string
		extra           map[string]interface{}
	}

	obstacleDetectorSlice := []motion.ObstacleDetectorName{
		{VisionServiceName: vision.Named("injectedVisionSvc"), CameraName: camera.Named("injectedCamera")},
	}

	cfg := &motion.MotionConfiguration{
		PositionPollingFreqHz: 1, ObstaclePollingFreqHz: 20, PlanDeviationMM: epsilonMM, ObstacleDetectors: obstacleDetectorSlice,
	}

	extra := map[string]interface{}{"max_replans": 10, "max_ik_solutions": 1, "smooth_iter": 1}
	extraNoReplan := map[string]interface{}{"max_replans": 0, "max_ik_solutions": 1, "smooth_iter": 1}

	// We set a flag here per test case so that detections are not returned the first time each vision service is called
	testCases := []testCase{
		{
			name: "ensure no replan from discovered obstacles",
			getPCfunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
				caseName := "test-case-1"
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: -1000, Y: -1000, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 10, Y: 10, Z: 10}, caseName)
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), caseName+"-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: true,
			extra:           extraNoReplan,
		},
		{
			name: "ensure replan due to obstacle collision",
			getPCfunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
				caseName := "test-case-2"
				// The camera is parented to the base. Thus, this will always see an obstacle 300mm in front of where the base is.
				// Note: for createMoveOnGlobeEnvironment, the camera is given an orientation such that it is pointing left, not
				// forwards. Thus, an obstacle in front of the base will be seen as being in +X.
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: 300, Y: 0, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 20, Y: 20, Z: 10}, caseName)
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), caseName+"-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: false,
			expectedErr:     fmt.Sprintf("exceeded maximum number of replans: %d: plan failed", 0),
			extra:           extraNoReplan,
		},
		{
			name: "ensure replan reaching goal",
			getPCfunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
				caseName := "test-case-3"
				// This base will always see an obstacle 800mm in front of it, triggering several replans.
				// However, enough replans should eventually get it to its goal.
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: 800, Y: 0, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 20, Y: 20, Z: 10}, caseName)
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), caseName+"-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: true,
			extra:           extra,
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()

		calledPC := false
		pcFunc := func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
			if !calledPC {
				calledPC = true
				return []*viz.Object{}, nil
			}
			return tc.getPCfunc(ctx, cameraName, extra)
		}

		_, ms, closeFunc := createMoveOnGlobeEnvironment(
			ctx,
			t,
			gpsOrigin,
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
			5000,
		)
		defer closeFunc(ctx)

		srvc, ok := ms.(*builtIn).visionServices[cfg.ObstacleDetectors[0].VisionServiceName].(*inject.VisionService)
		test.That(t, ok, test.ShouldBeTrue)
		srvc.GetObjectPointCloudsFunc = pcFunc

		req := motion.MoveOnGlobeReq{
			ComponentName:      resource.NewName(base.API, baseName),
			Destination:        dst,
			MovementSensorName: resource.NewName(movementsensor.API, moveSensorName),
			MotionCfg:          cfg,
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
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldEqual, tc.expectedErr)
		}
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
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

	kb, ms, closeFunc := createMoveOnMapEnvironment(
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
	req := motion.MoveOnMapReq{
		ComponentName: base.Named("test-base"),
		Destination:   spatialmath.NewPoseFromPoint(r3.Vector{X: 800, Y: 0, Z: 0}),
		SlamName:      slam.Named("test_slam"),
		MotionCfg: &motion.MotionConfiguration{
			PositionPollingFreqHz: 1, ObstaclePollingFreqHz: 40, PlanDeviationMM: 1, ObstacleDetectors: obstacleDetectorSlice,
		},
		Extra: map[string]interface{}{"smooth_iter": 20},
	}

	executionID, err := ms.MoveOnMap(ctx, req)
	test.That(t, err, test.ShouldBeNil)

	timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*15)
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
	_, err = ms.Move(context.Background(), gripper.Named("gr"), grabPose, nil, nil, nil)
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
			success, err := ms.Move(ctx, injectArmName, goal, nil, nil, extra)
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
				PlanDeviationMM:       10000,
				LinearMPerSec:         10,
				PositionPollingFreqHz: 4,
				ObstaclePollingFreqHz: 1,
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
			injectSlam := createInjectedSlam(slamName, "pointcloud/octagonspace.pcd", nil)

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
			injectSlam := createInjectedSlam(slamName, "pointcloud/octagonspace.pcd", nil)

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

func TestGetTransientDetections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, ms, closeFunc := createMoveOnMapEnvironment(
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
		f             spatialmath.Pose
		detectionPose spatialmath.Pose
	}
	testCases := []testCase{
		{
			name:          "relative - SLAM/base theta does not matter",
			f:             spatialmath.NewZeroPose(),
			detectionPose: spatialmath.NewPose(r3.Vector{4, 10, -8}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90}),
		},
		{
			name:          "absolute - SLAM theta: 0, base theta: -90 == 270",
			f:             spatialmath.NewPose(r3.Vector{-4, -10, 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -90}),
			detectionPose: spatialmath.NewPose(r3.Vector{6, -14, -8}, &spatialmath.OrientationVectorDegrees{OX: 1, Theta: -90}),
		},
		{
			name:          "absolute - SLAM theta: 90, base theta: 0",
			f:             spatialmath.NewPose(r3.Vector{-4, -10, 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0}),
			detectionPose: spatialmath.NewPose(r3.Vector{0, 0, -8}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90}),
		},
		{
			name:          "absolute - SLAM theta: 180, base theta: 90",
			f:             spatialmath.NewPose(r3.Vector{-4, -10, 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90}),
			detectionPose: spatialmath.NewPose(r3.Vector{-14, -6, -8}, &spatialmath.OrientationVectorDegrees{OX: -1, Theta: -90}),
		},
		{
			name:          "absolute - SLAM theta: 270, base theta: 180",
			f:             spatialmath.NewPose(r3.Vector{-4, -10, 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 180}),
			detectionPose: spatialmath.NewPose(r3.Vector{-8, -20, -8}, &spatialmath.OrientationVectorDegrees{OY: -1, Theta: -90}),
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		transformedGeoms, err := mr.getTransientDetections(ctx, injectedVis, camera.Named("test-camera"), tc.f)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, transformedGeoms.Parent(), test.ShouldEqual, referenceframe.World)
		test.That(t, len(transformedGeoms.Geometries()), test.ShouldEqual, 1)
		test.That(t, spatialmath.PoseAlmostEqual(transformedGeoms.Geometries()[0].Pose(), tc.detectionPose), test.ShouldBeTrue)
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testFn(t, c)
		})
	}
}

func TestStopPlan(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
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
	//nolint:dogsled
	_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
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
	//nolint:dogsled
	_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
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
	err := kb.GoToInputs(ctx, []referenceframe.Input{{0}, {0.001+math.Pi/2}, {0}, {91}})
	test.That(t, err, test.ShouldBeNil)
}

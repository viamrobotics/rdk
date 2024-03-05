package builtin

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
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

func TestMoveOnMapAskewIMUTestMoveOnMapAskewIMU(t *testing.T) {
	t.Parallel()
	extraPosOnly := map[string]interface{}{"smooth_iter": 5, "motion_profile": "position_only"}
	t.Run("Askew but valid base should be able to plan", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: 1, OY: 1, OZ: 1, Theta: 35}
		askewOrientCorrected := &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -22.988}
		// goal x-position of 1.32m is scaled to be in mm
		goal1SLAMFrame := spatialmath.NewPose(r3.Vector{X: 1.32 * 1000, Y: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 55})
		goal1BaseFrame := spatialmath.Compose(goal1SLAMFrame, motion.SLAMOrientationAdjustment)

		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, spatialmath.NewPoseFromOrientation(askewOrient))
		defer ms.Close(ctx)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   goal1SLAMFrame,
			SlamName:      slam.Named("test_slam"),
			Extra:         extraPosOnly,
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*15)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*15)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)

		endPIF, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		// We need to transform the endPos by the corrected orientation in order to properly place it, otherwise it will go off in +Z somewhere.
		// In a real robot this will be taken care of by gravity.
		correctedPose := spatialmath.NewPoseFromOrientation(askewOrientCorrected)
		endPos := spatialmath.Compose(correctedPose, spatialmath.PoseBetween(spatialmath.NewPoseFromOrientation(askewOrient), endPIF.Pose()))

		test.That(t, spatialmath.PoseAlmostEqualEps(endPos, goal1BaseFrame, 10), test.ShouldBeTrue)
	})
	t.Run("Upside down base should fail to plan", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: 1, OY: 1, OZ: -1, Theta: 55}
		// goal x-position of 1.32m is scaled to be in mm
		goal1SLAMFrame := spatialmath.NewPose(r3.Vector{X: 1.32 * 1000, Y: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 55})

		_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, spatialmath.NewPoseFromOrientation(askewOrient))
		defer ms.Close(ctx)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   goal1SLAMFrame,
			SlamName:      slam.Named("test_slam"),
			Extra:         extraPosOnly,
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*15)
		defer timeoutFn()
		_, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "base appears to be upside down, check your movement sensor")
	})
}

func TestPositionalReplanning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
		injectedMovementSensor, _, kb, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, spatialmath.NewPoseFromPoint(tc.noise), 5)
		defer ms.Close(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      kb.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
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
			t.Parallel()
			testFn(t, c)
		})
	}
}

func TestObstacleReplanningGlobe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gpsOrigin := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsOrigin.Lat(), gpsOrigin.Lng()+1e-5)
	epsilonMM := 15.

	type testCase struct {
		name            string
		getPCfunc       func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error)
		expectedSuccess bool
		expectedErr     string
	}

	obstacleDetectorSlice := []motion.ObstacleDetectorName{
		{VisionServiceName: vision.Named("injectedVisionSvc"), CameraName: camera.Named("injectedCamera")},
	}

	cfg := &motion.MotionConfiguration{
		PositionPollingFreqHz: 1, ObstaclePollingFreqHz: 100, PlanDeviationMM: epsilonMM, ObstacleDetectors: obstacleDetectorSlice,
	}

	extra := map[string]interface{}{"max_replans": 0, "max_ik_solutions": 1, "smooth_iter": 1}

	i := 0
	j := 0

	testCases := []testCase{
		{
			name: "ensure no replan from discovered obstacles",
			getPCfunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
				if j == 0 {
					j++
					return []*viz.Object{}, nil
				}
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: -1000, Y: -1000, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 10, Y: 10, Z: 10}, "test-case-2")
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), "test-case-2-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: true,
		},
		{
			name: "ensure replan due to obstacle collision",
			getPCfunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
				if i == 0 {
					i++
					return []*viz.Object{}, nil
				}
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: 300, Y: 0, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 20, Y: 20, Z: 10}, "test-case-1")
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), "test-case-1-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: false,
			expectedErr:     fmt.Sprintf("exceeded maximum number of replans: %d: plan failed", 0),
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		injectedMovementSensor, _, kb, ms := createMoveOnGlobeEnvironment(
			ctx,
			t,
			gpsOrigin,
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
			5000,
		)
		defer ms.Close(ctx)

		srvc, ok := ms.(*builtIn).visionServices[cfg.ObstacleDetectors[0].VisionServiceName].(*inject.VisionService)
		test.That(t, ok, test.ShouldBeTrue)
		srvc.GetObjectPointCloudsFunc = tc.getPCfunc

		req := motion.MoveOnGlobeReq{
			ComponentName:      kb.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			MotionCfg:          cfg,
			Extra:              extra,
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
	t.Skip()
	cameraToBase := spatialmath.NewPose(r3.Vector{0, 0, 0}, &spatialmath.OrientationVectorDegrees{OY: 1, Theta: -90})
	cameraToBaseInv := spatialmath.PoseInverse(cameraToBase)

	ctx := context.Background()
	origin := spatialmath.NewPose(
		r3.Vector{X: -0.99503e3, Y: 0, Z: 0},
		&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -90},
	)

	boxWrld, err := spatialmath.NewBox(
		spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
		r3.Vector{X: 50, Y: 50, Z: 50}, "box-obstacle",
	)
	test.That(t, err, test.ShouldBeNil)

	kb, ms := createMoveOnMapEnvironment(
		ctx, t,
		"pointcloud/cardboardOcto.pcd",
		50, origin,
	)
	defer ms.Close(ctx)

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

		relativeBox := boxWrld.Transform(spatialmath.PoseInverse(currentPif.Pose())).Transform(cameraToBaseInv)
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
			PositionPollingFreqHz: 1, ObstaclePollingFreqHz: 100, PlanDeviationMM: 1, ObstacleDetectors: obstacleDetectorSlice,
		},
		Extra: map[string]interface{}{
			"max_replans": 2,
			"smooth_iter": 0,
		},
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

	plansWithStatus, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{
		ComponentName: base.Named("test-base"),
		LastPlanOnly:  false,
		ExecutionID:   executionID,
	})
	test.That(t, err, test.ShouldBeNil)
	populatedReplanReason := 0
	for _, planStatus := range plansWithStatus {
		for _, history := range planStatus.StatusHistory {
			if history.Reason != nil {
				populatedReplanReason++
			}
		}
	}
	test.That(t, populatedReplanReason, test.ShouldBeGreaterThanOrEqualTo, 1)
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

func TestMoveOnGlobe(t *testing.T) {
	ctx := context.Background()
	// Near antarctica 🐧
	gpsPoint := geo.NewPoint(-70, 40)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+7e-5)
	expectedDst := r3.Vector{X: 2662.16, Y: 0, Z: 0} // Relative pose to the starting point of the base; facing north, Y = forwards
	epsilonMM := 15.
	// create motion config
	extra := map[string]interface{}{
		"motion_profile": "position_only",
		"timeout":        5.,
		"smooth_iter":    5.,
	}

	t.Run("Changes to executions show up in PlanHistory", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Destination:        dst,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotBeEmpty)

		// returns the execution just created in the history
		ph, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ph), test.ShouldEqual, 1)
		test.That(t, ph[0].Plan.ExecutionID, test.ShouldResemble, executionID)
		test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, len(ph[0].Plan.Path()), test.ShouldNotEqual, 0)

		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: fakeBase.Name()})
		test.That(t, err, test.ShouldBeNil)

		ph2, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ph2), test.ShouldEqual, 1)
		test.That(t, ph2[0].Plan.ExecutionID, test.ShouldResemble, executionID)
		test.That(t, len(ph2[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, ph2[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateStopped)
		test.That(t, ph2[0].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, len(ph2[0].Plan.Path()), test.ShouldNotEqual, 0)

		// Proves that calling StopPlan after the plan has reached a terminal state is idempotent
		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: fakeBase.Name()})
		test.That(t, err, test.ShouldBeNil)
		ph3, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ph3, test.ShouldResemble, ph2)
	})

	t.Run("is able to reach a nearby geo point with empty values", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested NaN heading", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            math.NaN(),
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested positive heading", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            10000000,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested negative heading", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            -10000000,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point when the motion configuration is empty", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{},
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point when the motion configuration nil", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("ensure success to a nearby geo point", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			Obstacles:          []*spatialmath.GeoObstacle{},
			MotionCfg:          motionCfg,
			Extra:              extra,
		}
		planExecutor, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)

		mr, ok := planExecutor.(*moveRequest)
		test.That(t, ok, test.ShouldBeTrue)

		test.That(t, mr.planRequest.Goal.Pose().Point().X, test.ShouldAlmostEqual, expectedDst.X, epsilonMM)
		test.That(t, mr.planRequest.Goal.Pose().Point().Y, test.ShouldAlmostEqual, expectedDst.Y, epsilonMM)

		planResp, err := mr.Plan(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(planResp.Path()), test.ShouldBeGreaterThan, 2)

		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("go around an obstacle", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}

		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 50, Y: 0, Z: 0})
		boxDims := r3.Vector{X: 5, Y: 50, Z: 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})
		startPose, err := fakeBase.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			Obstacles:          []*spatialmath.GeoObstacle{geoObstacle},
			MotionCfg:          motionCfg,
			Extra:              extra,
		}
		mr, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)
		planResp, err := mr.Plan(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(planResp.Path()), test.ShouldBeGreaterThan, 2)

		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)

		endPose, err := fakeBase.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		movedPose := spatialmath.PoseBetween(startPose.Pose(), endPose.Pose())
		test.That(t, movedPose.Point().X, test.ShouldAlmostEqual, expectedDst.X, epsilonMM)
		test.That(t, movedPose.Point().Y, test.ShouldAlmostEqual, expectedDst.Y, epsilonMM)
	})

	t.Run("fail because of obstacle", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)

		// Construct a set of obstacles that entirely enclose the goal point
		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 250, Y: 0, Z: 0})
		boxDims := r3.Vector{X: 20, Y: 6660, Z: 10}
		geometry1, err := spatialmath.NewBox(boxPose, boxDims, "wall1")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{X: 5000, Y: 0, Z: 0})
		boxDims = r3.Vector{X: 20, Y: 6660, Z: 10}
		geometry2, err := spatialmath.NewBox(boxPose, boxDims, "wall2")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{X: 2500, Y: 2500, Z: 0})
		boxDims = r3.Vector{X: 6660, Y: 20, Z: 10}
		geometry3, err := spatialmath.NewBox(boxPose, boxDims, "wall3")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{X: 2500, Y: -2500, Z: 0})
		boxDims = r3.Vector{X: 6660, Y: 20, Z: 10}
		geometry4, err := spatialmath.NewBox(boxPose, boxDims, "wall4")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometry1, geometry2, geometry3, geometry4})

		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			Obstacles:          []*spatialmath.GeoObstacle{geoObstacle},
			MotionCfg:          &motion.MotionConfiguration{},
			Extra:              extra,
		}
		moveRequest, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)
		planResp, err := moveRequest.Plan(ctx)
		test.That(t, err, test.ShouldBeError)
		test.That(t, planResp, test.ShouldBeNil)
	})

	t.Run("check offset constructed correctly", func(t *testing.T) {
		_, fsSvc, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		baseOrigin := referenceframe.NewPoseInFrame("test-base", spatialmath.NewZeroPose())
		movementSensorToBase, err := fsSvc.TransformPose(ctx, baseOrigin, "test-gps", nil)
		if err != nil {
			movementSensorToBase = baseOrigin
		}
		test.That(t, movementSensorToBase.Pose().Point(), test.ShouldResemble, r3.Vector{X: 10, Y: 0, Z: 0})
	})
}

func TestMoveOnMapStaticObs(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	extra := map[string]interface{}{
		"motion_profile": "position_only",
		"timeout":        5.,
		"smooth_iter":    10.,
	}

	baseName := "test-base"
	slamName := "test-slam"

	// Create an injected Base
	geometry, err := (&spatialmath.GeometryConfig{R: 30}).ParseConfig()
	test.That(t, err, test.ShouldBeNil)

	injectBase := inject.NewBase(baseName)
	injectBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
		return []spatialmath.Geometry{geometry}, nil
	}
	injectBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
		return base.Properties{TurningRadiusMeters: 0, WidthMeters: 0.6}, nil
	}

	// Create a base link
	baseLink := createBaseLink(t)

	// Create an injected SLAM
	injectSlam := createInjectedSlam(slamName, "pointcloud/octagonspace.pcd", nil)
	injectSlam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		return spatialmath.NewPose(
			r3.Vector{X: 0.58772e3, Y: -0.80826e3, Z: 0},
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90},
		), "", nil
	}

	// Create a motion service
	deps := resource.Dependencies{injectBase.Name(): injectBase, injectSlam.Name(): injectSlam}
	fsParts := []*referenceframe.FrameSystemPart{{FrameConfig: baseLink}}

	ms, err := NewBuiltIn(ctx, deps, resource.Config{ConvertedAttributes: &Config{}}, logger)
	test.That(t, err, test.ShouldBeNil)
	defer ms.Close(context.Background())

	fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)
	ms.(*builtIn).fsService = fsSvc

	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.6556e3, Y: 0.64152e3})

	req := motion.MoveOnMapReq{
		ComponentName: injectBase.Name(),
		Destination:   goal,
		SlamName:      injectSlam.Name(),
		MotionCfg:     &motion.MotionConfiguration{PlanDeviationMM: 0.01},
		Extra:         extra,
	}

	t.Run("one obstacle", func(t *testing.T) {
		// WTS: static obstacles are obeyed at plan time.

		// We place an obstacle on the left side of the robot to force our motion planner to return a path
		// which veers to the right. We then place an obstacle to the right of the robot and project the
		// robot's position across the path. By showing that we have a collision on the path with an
		// obstacle on the right we prove that our path does not collide with the original obstacle
		// placed on the left.
		obstacleLeft, err := spatialmath.NewBox(
			spatialmath.NewPose(r3.Vector{X: 0.22981e3, Y: -0.38875e3, Z: 0},
				&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 45}),
			r3.Vector{X: 900, Y: 10, Z: 10},
			"obstacleLeft",
		)
		test.That(t, err, test.ShouldBeNil)

		req.Obstacles = []spatialmath.Geometry{obstacleLeft}

		// construct move request
		planExecutor, err := ms.(*builtIn).newMoveOnMapRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)
		mr, ok := planExecutor.(*moveRequest)
		test.That(t, ok, test.ShouldBeTrue)

		// construct plan
		plan, err := mr.Plan(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(plan.Path()), test.ShouldBeGreaterThan, 2)

		// place obstacle in opposte position and show that the generate path
		// collides with obstacleRight
		obstacleRight, err := spatialmath.NewBox(
			spatialmath.NewPose(r3.Vector{0.89627e3, -0.37192e3, 0},
				&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -45}),
			r3.Vector{900, 10, 10},
			"obstacleRight",
		)
		test.That(t, err, test.ShouldBeNil)

		wrldSt, err := referenceframe.NewWorldState(
			[]*referenceframe.GeometriesInFrame{
				referenceframe.NewGeometriesInFrame(
					referenceframe.World,
					[]spatialmath.Geometry{obstacleRight},
				),
			}, nil,
		)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(
			mr.planRequest.Frame,
			plan,
			wrldSt,
			mr.planRequest.FrameSystem,
			spatialmath.NewPose(
				r3.Vector{X: 0.58772e3, Y: -0.80826e3, Z: 0},
				&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0},
			),
			referenceframe.StartPositions(mr.planRequest.FrameSystem),
			spatialmath.NewZeroPose(),
			lookAheadDistanceMM,
			logger,
		)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail due to obstacles enclosing goals", func(t *testing.T) {
		// define static obstacles
		obstacleTop, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0.64603e3, Y: 0.77151e3, Z: 0}),
			r3.Vector{X: 400, Y: 10, Z: 10},
			"obstacleTop",
		)
		test.That(t, err, test.ShouldBeNil)

		obstacleBottom, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0.64603e3, Y: 0.42479e3, Z: 0}),
			r3.Vector{X: 400, Y: 10, Z: 10},
			"obstacleBottom",
		)
		test.That(t, err, test.ShouldBeNil)

		obstacleLeft, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0.47525e3, Y: 0.65091e3, Z: 0}),
			r3.Vector{X: 10, Y: 400, Z: 10},
			"obstacleLeft",
		)
		test.That(t, err, test.ShouldBeNil)

		obstacleRight, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0.82183e3, Y: 0.64589e3, Z: 0}),
			r3.Vector{X: 10, Y: 400, Z: 10},
			"obstacleRight",
		)
		test.That(t, err, test.ShouldBeNil)

		req.Obstacles = []spatialmath.Geometry{obstacleTop, obstacleBottom, obstacleLeft, obstacleRight}

		// construct move request
		planExecutor, err := ms.(*builtIn).newMoveOnMapRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)
		mr, ok := planExecutor.(*moveRequest)
		test.That(t, ok, test.ShouldBeTrue)

		// construct plan
		_, err = mr.Plan(ctx)
		test.That(t, err, test.ShouldBeError, errors.New("context deadline exceeded"))
	})
}

func TestMoveOnMap(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("Long distance", func(t *testing.T) {
		if runtime.GOARCH == "arm" {
			t.Skip("skipping on 32-bit ARM, large maps use too much memory")
		}
		extra := map[string]interface{}{"smooth_iter": 0, "motion_profile": "position_only"}
		// goal position is scaled to be in mm
		goalInBaseFrame := spatialmath.NewPoseFromPoint(r3.Vector{X: -32.508 * 1000, Y: -2.092 * 1000})
		goalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, goalInBaseFrame)

		kb, ms := createMoveOnMapEnvironment(
			ctx,
			t,
			"slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd",
			110,
			spatialmath.NewPoseFromPoint(r3.Vector{0, -1600, 0}),
		)
		defer ms.Close(ctx)
		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   goalInSLAMFrame,
			SlamName:      slam.Named("test_slam"),
			Extra:         extra,
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*90)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*35)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)

		endPos, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, spatialmath.PoseAlmostCoincidentEps(endPos.Pose(), goalInBaseFrame, 15), test.ShouldBeTrue)
	})

	t.Run("Plans", func(t *testing.T) {
		// goal x-position of 1.32m is scaled to be in mm
		// Orientation theta should be at least 3 degrees away from an integer multiple of 22.5 to ensure the position-only test functions.
		goalInBaseFrame := spatialmath.NewPose(r3.Vector{X: 1.32 * 1000, Y: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 33})
		goalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, goalInBaseFrame)
		extra := map[string]interface{}{"smooth_iter": 0}
		extraPosOnly := map[string]interface{}{"smooth_iter": 5, "motion_profile": "position_only"}

		t.Run("ensure success of movement around obstacle", func(t *testing.T) {
			kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   goalInSLAMFrame,
				SlamName:      slam.Named("test_slam"),
				Extra:         extra,
			}

			timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

			timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
				ComponentName: req.ComponentName,
				ExecutionID:   executionID,
				LastPlanOnly:  true,
			})
			test.That(t, err, test.ShouldBeNil)

			endPos, err := kb.CurrentPosition(ctx)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, spatialmath.PoseAlmostCoincidentEps(endPos.Pose(), goalInBaseFrame, 15), test.ShouldBeTrue)
		})
		t.Run("check that straight line path executes", func(t *testing.T) {
			kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)
			easyGoalInBaseFrame := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.277 * 1000, Y: 0.593 * 1000})
			easyGoalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, easyGoalInBaseFrame)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   easyGoalInSLAMFrame,
				MotionCfg: &motion.MotionConfiguration{
					PlanDeviationMM: 1,
				},
				SlamName: slam.Named("test_slam"),
				Extra:    extra,
			}

			timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

			timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
				ComponentName: req.ComponentName,
				ExecutionID:   executionID,
				LastPlanOnly:  true,
			})
			test.That(t, err, test.ShouldBeNil)

			endPos, err := kb.CurrentPosition(ctx)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), easyGoalInBaseFrame, 10), test.ShouldBeTrue)
		})

		t.Run("check that position-only mode executes", func(t *testing.T) {
			kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   goalInSLAMFrame,
				SlamName:      slam.Named("test_slam"),
				Extra:         extraPosOnly,
			}

			timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

			timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*5)
			defer timeoutFn()
			err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
				ComponentName: req.ComponentName,
				ExecutionID:   executionID,
				LastPlanOnly:  true,
			})
			test.That(t, err, test.ShouldBeNil)

			endPos, err := kb.CurrentPosition(ctx)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, spatialmath.PoseAlmostCoincidentEps(endPos.Pose(), goalInBaseFrame, 15), test.ShouldBeTrue)
			// Position only mode should not yield the goal orientation.
			test.That(t, spatialmath.OrientationAlmostEqualEps(
				endPos.Pose().Orientation(),
				goalInBaseFrame.Orientation(),
				0.05), test.ShouldBeFalse)
		})

		t.Run("should fail due to map collision", func(t *testing.T) {
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -500}))
			defer ms.Close(ctx)
			easyGoalInBaseFrame := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.277 * 1000, Y: 0.593 * 1000})
			easyGoalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, easyGoalInBaseFrame)
			executionID, err := ms.MoveOnMap(
				context.Background(),
				motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   easyGoalInSLAMFrame,
					SlamName:      slam.Named("test_slam"),
					Extra:         extra,
				},
			)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, strings.Contains(err.Error(), "starting collision between SLAM map and "), test.ShouldBeTrue)
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})
	})

	t.Run("Subsequent", func(t *testing.T) {
		// goal x-position of 1.32m is scaled to be in mm
		goal1SLAMFrame := spatialmath.NewPose(r3.Vector{X: 1.32 * 1000, Y: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 55})
		goal1BaseFrame := spatialmath.Compose(goal1SLAMFrame, motion.SLAMOrientationAdjustment)
		goal2SLAMFrame := spatialmath.NewPose(r3.Vector{X: 277, Y: 593}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 150})
		goal2BaseFrame := spatialmath.Compose(goal2SLAMFrame, motion.SLAMOrientationAdjustment)

		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
		defer ms.Close(ctx)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   goal1SLAMFrame,
			SlamName:      slam.Named("test_slam"),
			Extra:         map[string]interface{}{"smooth_iter": 5},
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)

		endPos, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		logger.Debug(spatialmath.PoseToProtobuf(endPos.Pose()))
		test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), goal1BaseFrame, 10), test.ShouldBeTrue)

		// Now, we try to go to the second goal. Since the `CurrentPosition` of our base is at `goal1`, the pose that motion solves for and
		// logs should be {x:-1043  y:593}
		req = motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   goal2SLAMFrame,
			SlamName:      slam.Named("test_slam"),
			Extra:         map[string]interface{}{"smooth_iter": 5},
		}
		timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		executionID, err = ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn = context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)

		endPos, err = kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), goal2BaseFrame, 5), test.ShouldBeTrue)

		plans, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{
			ComponentName: base.Named("test-base"),
			LastPlanOnly:  false,
			ExecutionID:   executionID,
		})
		test.That(t, err, test.ShouldBeNil)

		goalPose1 := plans[0].Plan.Path()[0]["test-base"].Pose()
		goalPose2 := spatialmath.PoseBetween(
			plans[0].Plan.Path()[0]["test-base"].Pose(),
			plans[0].Plan.Path()[len(plans[0].Plan.Path())-1]["test-base"].Pose(),
		)

		// We don't actually surface the internal motion planning goal; we report to the user in terms of what the user provided us.
		// Thus, we use PlanHistory to get the plan steps of the latest plan.
		// The zeroth index of the plan steps is the relative position of goal1 and the pose inverse between the first and last value of
		// plan steps gives us the relative pose we solved for goal2.
		test.That(t, spatialmath.PoseAlmostEqualEps(goalPose1, goal1BaseFrame, 10), test.ShouldBeTrue)

		// This is the important test.
		test.That(t, spatialmath.PoseAlmostEqualEps(goalPose2, spatialmath.PoseBetween(goal1BaseFrame, goal2BaseFrame), 10), test.ShouldBeTrue)
	})

	t.Run("Timeout", func(t *testing.T) {
		cfg, err := config.Read(ctx, "../data/real_wheeled_base.json", logger)
		test.That(t, err, test.ShouldBeNil)
		myRobot, err := robotimpl.New(ctx, cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, myRobot.Close(context.Background()), test.ShouldBeNil)
		}()

		injectSlam := createInjectedSlam("test_slam", "pointcloud/octagonspace.pcd", nil)

		realBase, err := base.FromRobot(myRobot, "test-base")
		test.That(t, err, test.ShouldBeNil)

		deps := resource.Dependencies{
			injectSlam.Name(): injectSlam,
			realBase.Name():   realBase,
		}
		fsParts := []*referenceframe.FrameSystemPart{
			{FrameConfig: createBaseLink(t)},
		}

		conf := resource.Config{ConvertedAttributes: &Config{}}
		ms, err := NewBuiltIn(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldBeNil)
		defer ms.Close(context.Background())

		fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
		test.That(t, err, test.ShouldBeNil)
		ms.(*builtIn).fsService = fsSvc

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   spatialmath.NewPoseFromPoint(r3.Vector{X: 1001, Y: 1001}),
			SlamName:      slam.Named("test_slam"),
			Extra:         map[string]interface{}{"timeout": 0.01},
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("Changes to executions show up in PlanHistory", func(t *testing.T) {
		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
		defer ms.Close(ctx)
		easyGoalInBaseFrame := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.277 * 1000, Y: 0.593 * 1000})
		easyGoalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, easyGoalInBaseFrame)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   easyGoalInSLAMFrame,
			MotionCfg: &motion.MotionConfiguration{
				PlanDeviationMM: 1,
			},
			SlamName: slam.Named("test_slam"),
			Extra:    map[string]interface{}{"smooth_iter": 0},
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotBeEmpty)

		// returns the execution just created in the history
		ph, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ph), test.ShouldEqual, 1)
		test.That(t, ph[0].Plan.ExecutionID, test.ShouldResemble, executionID)
		test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, len(ph[0].Plan.Path()), test.ShouldNotEqual, 0)

		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: kb.Name()})
		test.That(t, err, test.ShouldBeNil)

		ph2, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ph2), test.ShouldEqual, 1)
		test.That(t, ph2[0].Plan.ExecutionID, test.ShouldResemble, executionID)
		test.That(t, len(ph2[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, ph2[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateStopped)
		test.That(t, ph2[0].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, len(ph2[0].Plan.Path()), test.ShouldNotEqual, 0)

		// Proves that calling StopPlan after the plan has reached a terminal state is idempotent
		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: kb.Name()})
		test.That(t, err, test.ShouldBeNil)
		ph3, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ph3, test.ShouldResemble, ph2)
	})

	t.Run("returns error when within plan dev m of goal with position_only", func(t *testing.T) {
		_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
		defer ms.Close(ctx)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   spatialmath.NewZeroPose(),
			SlamName:      slam.Named("test_slam"),
			MotionCfg:     &motion.MotionConfiguration{},
			Extra:         map[string]interface{}{"motion_profile": "position_only"},
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeError, errors.New("no need to move, already within planDeviationMM"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("pass when within plan dev m of goal without position_only due to theta difference in goal", func(t *testing.T) {
		_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
		defer ms.Close(ctx)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   spatialmath.NewZeroPose(),
			SlamName:      slam.Named("test_slam"),
			MotionCfg:     &motion.MotionConfiguration{},
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotBeEmpty)
	})
}

func TestMoveCallInputs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("MoveOnMap", func(t *testing.T) {
		t.Parallel()
		t.Run("Returns error when called with an unknown component", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("non existent base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("Resource missing from dependencies. Resource: rdk:component:base/non existent base"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns error when destination is nil", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				SlamName:      slam.Named("test_slam"),
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("destination cannot be nil"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error if the base provided is not a base", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: slam.Named("test_slam"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("Resource missing from dependencies. Resource: rdk:service:slam/test_slam"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error if the slamName provided is not SLAM", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: slam.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test-base"),
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("Resource missing from dependencies. Resource: rdk:service:slam/test-base"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when motion configuration has a negative PlanDeviationMM", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{PlanDeviationMM: -1},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when motion configuration has a NaN PlanDeviationMM", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{PlanDeviationMM: math.NaN()},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when the motion configuration has a negative ObstaclePollingFreqHz", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{ObstaclePollingFreqHz: -1},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when the motion configuration has a NaN ObstaclePollingFreqHz", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{ObstaclePollingFreqHz: math.NaN()},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when the motion configuration has a negative PositionPollingFreqHz", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{PositionPollingFreqHz: -1},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when the motion configuration has a NaN PositionPollingFreqHz", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{PositionPollingFreqHz: math.NaN()},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when motion configuration has a negative AngularDegsPerSec", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{AngularDegsPerSec: -1},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when motion configuration has a NaN AngularDegsPerSec", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{AngularDegsPerSec: math.NaN()},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when motion configuration has a negative LinearMPerSec", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{LinearMPerSec: -1},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("Returns an error when motion configuration has a NaN LinearMPerSec", func(t *testing.T) {
			t.Parallel()
			_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
			defer ms.Close(ctx)

			req := motion.MoveOnMapReq{
				ComponentName: base.Named("test-base"),
				Destination:   spatialmath.NewZeroPose(),
				SlamName:      slam.Named("test_slam"),
				MotionCfg:     &motion.MotionConfiguration{LinearMPerSec: math.NaN()},
			}

			executionID, err := ms.(*builtIn).MoveOnMap(context.Background(), req)
			test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("collision_buffer_mm validtations", func(t *testing.T) {
			t.Run("fail when collision_buffer_mm is not a float", func(t *testing.T) {
				_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
				defer ms.Close(ctx)

				req := motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      slam.Named("test_slam"),
					MotionCfg:     &motion.MotionConfiguration{},
					Extra:         map[string]interface{}{"collision_buffer_mm": "not a float"},
				}

				timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
				defer timeoutFn()
				executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
				test.That(t, err, test.ShouldBeError, errors.New("could not interpret collision_buffer_mm field as float64"))
				test.That(t, executionID, test.ShouldNotBeEmpty)
			})

			t.Run("fail when collision_buffer_mm is negative", func(t *testing.T) {
				_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
				defer ms.Close(ctx)

				req := motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      slam.Named("test_slam"),
					MotionCfg:     &motion.MotionConfiguration{},
					Extra:         map[string]interface{}{"collision_buffer_mm": -1.},
				}

				timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
				defer timeoutFn()
				executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
				test.That(t, err, test.ShouldBeError, errors.New("collision_buffer_mm can't be negative"))
				test.That(t, executionID, test.ShouldResemble, uuid.Nil)
			})

			t.Run("fail when collisions are predicted within the collision buffer", func(t *testing.T) {
				_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
				defer ms.Close(ctx)

				req := motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      slam.Named("test_slam"),
					MotionCfg:     &motion.MotionConfiguration{},
					Extra:         map[string]interface{}{"collision_buffer_mm": 200.},
				}

				timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
				defer timeoutFn()
				executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
				test.That(t, strings.Contains(err.Error(), "starting collision between SLAM map and "), test.ShouldBeTrue)
				test.That(t, executionID, test.ShouldResemble, uuid.Nil)
			})

			t.Run("pass when collision_buffer_mm is a small positive float", func(t *testing.T) {
				_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
				defer ms.Close(ctx)

				req := motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      slam.Named("test_slam"),
					MotionCfg:     &motion.MotionConfiguration{},
					Extra:         map[string]interface{}{"collision_buffer_mm": 1e-5},
				}

				timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
				defer timeoutFn()
				executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})

			t.Run("pass when collision_buffer_mm is a positive float", func(t *testing.T) {
				_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
				defer ms.Close(ctx)

				req := motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      slam.Named("test_slam"),
					MotionCfg:     &motion.MotionConfiguration{},
					Extra:         map[string]interface{}{"collision_buffer_mm": 0.1},
				}

				timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
				defer timeoutFn()
				executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})

			t.Run("pass when extra is empty", func(t *testing.T) {
				_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
				defer ms.Close(ctx)

				req := motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      slam.Named("test_slam"),
					MotionCfg:     &motion.MotionConfiguration{},
					Extra:         map[string]interface{}{},
				}

				timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
				defer timeoutFn()
				executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})

			t.Run("passes validations when extra is nil", func(t *testing.T) {
				_, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
				defer ms.Close(ctx)

				req := motion.MoveOnMapReq{
					ComponentName: base.Named("test-base"),
					Destination:   spatialmath.NewZeroPose(),
					SlamName:      slam.Named("test_slam"),
					MotionCfg:     &motion.MotionConfiguration{},
				}

				timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
				defer timeoutFn()
				executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})
		})
	})

	t.Run("MoveOnGlobe", func(t *testing.T) {
		t.Parallel()
		// Near antarctica 🐧
		gpsPoint := geo.NewPoint(-70, 40)
		dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-4)
		// create motion config
		extra := map[string]interface{}{
			"motion_profile": "position_only",
			"timeout":        5.,
			"smooth_iter":    5.,
		}
		t.Run("returns error when called with an unknown component", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      base.Named("non existent base"),
				MovementSensorName: injectedMovementSensor.Name(),
				Destination:        geo.NewPoint(0, 0),
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:base/non existent base\" not found"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns error when called with an unknown movement sensor", func(t *testing.T) {
			t.Parallel()
			_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: movementsensor.Named("non existent movement sensor"),
				Destination:        geo.NewPoint(0, 0),
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			e := "Resource missing from dependencies. Resource: rdk:component:movement_sensor/non existent movement sensor"
			test.That(t, err, test.ShouldBeError, errors.New(e))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns error when request would require moving more than 5 km", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Destination:        geo.NewPoint(0, 0),
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("cannot move more than 5 kilometers"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns error when destination is nil", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("destination cannot be nil"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns error when destination contains NaN", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)

			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
			}

			dests := []*geo.Point{
				geo.NewPoint(math.NaN(), math.NaN()),
				geo.NewPoint(0, math.NaN()),
				geo.NewPoint(math.NaN(), 0),
			}

			for _, d := range dests {
				req.Destination = d
				executionID, err := ms.MoveOnGlobe(ctx, req)
				test.That(t, err, test.ShouldBeError, errors.New("destination may not contain NaN"))
				test.That(t, executionID, test.ShouldResemble, uuid.Nil)
			}
		})

		t.Run("returns an error if the base provided is not a base", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      injectedMovementSensor.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				Extra:              extra,
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:movement_sensor/test-gps\" not found"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error if the movement_sensor provided is not a movement_sensor", func(t *testing.T) {
			t.Parallel()
			_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: fakeBase.Name(),
				Heading:            90,
				Destination:        dst,
				Extra:              extra,
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("Resource missing from dependencies. Resource: rdk:component:base/test-base"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("errors when motion configuration has a negative PlanDeviationMM", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{PlanDeviationMM: -1},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("errors when motion configuration has a NaN PlanDeviationMM", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{PlanDeviationMM: math.NaN()},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when the motion configuration has a negative ObstaclePollingFreqHz", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{ObstaclePollingFreqHz: -1},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when the motion configuration has a NaN ObstaclePollingFreqHz", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{ObstaclePollingFreqHz: math.NaN()},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when the motion configuration has a negative PositionPollingFreqHz", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{PositionPollingFreqHz: -1},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when the motion configuration has a NaN PositionPollingFreqHz", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{PositionPollingFreqHz: math.NaN()},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when motion configuration has a negative AngularDegsPerSec", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{AngularDegsPerSec: -1},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when motion configuration has a NaN AngularDegsPerSec", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{AngularDegsPerSec: math.NaN()},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when motion configuration has a negative LinearMPerSec", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{LinearMPerSec: -1},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be negative"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("returns an error when motion configuration has a NaN LinearMPerSec", func(t *testing.T) {
			t.Parallel()
			injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
			defer ms.Close(ctx)
			req := motion.MoveOnGlobeReq{
				ComponentName:      fakeBase.Name(),
				MovementSensorName: injectedMovementSensor.Name(),
				Heading:            90,
				Destination:        dst,
				MotionCfg:          &motion.MotionConfiguration{LinearMPerSec: math.NaN()},
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		})

		t.Run("collision_buffer_mm validtations", func(t *testing.T) {
			t.Run("fail when collision_buffer_mm is not a float", func(t *testing.T) {
				injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
				defer ms.Close(ctx)
				req := motion.MoveOnGlobeReq{
					ComponentName:      fakeBase.Name(),
					MovementSensorName: injectedMovementSensor.Name(),
					Heading:            90,
					Destination:        dst,
					MotionCfg:          &motion.MotionConfiguration{},
					Extra:              map[string]interface{}{"collision_buffer_mm": "not a float"},
				}
				executionID, err := ms.MoveOnGlobe(ctx, req)
				test.That(t, err, test.ShouldBeError, errors.New("could not interpret collision_buffer_mm field as float64"))
				test.That(t, executionID, test.ShouldResemble, uuid.Nil)
			})

			t.Run("fail when collision_buffer_mm is negative", func(t *testing.T) {
				injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
				defer ms.Close(ctx)
				req := motion.MoveOnGlobeReq{
					ComponentName:      fakeBase.Name(),
					MovementSensorName: injectedMovementSensor.Name(),
					Heading:            90,
					Destination:        dst,
					MotionCfg:          &motion.MotionConfiguration{},
					Extra:              map[string]interface{}{"collision_buffer_mm": -1.},
				}
				executionID, err := ms.MoveOnGlobe(ctx, req)
				test.That(t, err, test.ShouldBeError, errors.New("collision_buffer_mm can't be negative"))
				test.That(t, executionID, test.ShouldResemble, uuid.Nil)
			})

			t.Run("pass when collision_buffer_mm is a small positive float", func(t *testing.T) {
				injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
				defer ms.Close(ctx)
				req := motion.MoveOnGlobeReq{
					ComponentName:      fakeBase.Name(),
					MovementSensorName: injectedMovementSensor.Name(),
					Heading:            90,
					Destination:        dst,
					MotionCfg:          &motion.MotionConfiguration{},
					Extra:              map[string]interface{}{"collision_buffer_mm": 1e-5},
				}
				executionID, err := ms.MoveOnGlobe(ctx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})

			t.Run("pass when collision_buffer_mm is a positive float", func(t *testing.T) {
				injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
				defer ms.Close(ctx)
				req := motion.MoveOnGlobeReq{
					ComponentName:      fakeBase.Name(),
					MovementSensorName: injectedMovementSensor.Name(),
					Heading:            90,
					Destination:        dst,
					MotionCfg:          &motion.MotionConfiguration{},
					Extra:              map[string]interface{}{"collision_buffer_mm": 10.},
				}
				executionID, err := ms.MoveOnGlobe(ctx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})

			t.Run("pass when extra is empty", func(t *testing.T) {
				injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
				defer ms.Close(ctx)
				req := motion.MoveOnGlobeReq{
					ComponentName:      fakeBase.Name(),
					MovementSensorName: injectedMovementSensor.Name(),
					Heading:            90,
					Destination:        dst,
					MotionCfg:          &motion.MotionConfiguration{},
					Extra:              map[string]interface{}{},
				}
				executionID, err := ms.MoveOnGlobe(ctx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})

			t.Run("passes validations when extra is nil", func(t *testing.T) {
				injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
				defer ms.Close(ctx)
				req := motion.MoveOnGlobeReq{
					ComponentName:      fakeBase.Name(),
					MovementSensorName: injectedMovementSensor.Name(),
					Heading:            90,
					Destination:        dst,
					MotionCfg:          &motion.MotionConfiguration{},
				}
				executionID, err := ms.MoveOnGlobe(ctx, req)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
			})
		})
	})
}

func TestGetTransientDetections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, ms := createMoveOnMapEnvironment(
		ctx, t,
		"slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd",
		100, spatialmath.NewZeroPose(),
	)
	t.Cleanup(func() { ms.Close(ctx) })

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
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
	defer ms.Close(ctx)

	req := motion.StopPlanReq{}
	err := ms.StopPlan(ctx, req)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(req.ComponentName))
}

func TestListPlanStatuses(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
	defer ms.Close(ctx)

	req := motion.ListPlanStatusesReq{}
	// returns no results as no move on globe calls have been made
	planStatusesWithIDs, err := ms.ListPlanStatuses(ctx, req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planStatusesWithIDs), test.ShouldEqual, 0)
}

func TestPlanHistory(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
	defer ms.Close(ctx)
	req := motion.PlanHistoryReq{}
	history, err := ms.PlanHistory(ctx, req)
	test.That(t, err, test.ShouldResemble, resource.NewNotFoundError(req.ComponentName))
	test.That(t, history, test.ShouldBeNil)
}

func TestNewValidatedMotionCfg(t *testing.T) {
	t.Run("returns expected defaults when given nil cfg for requestTypeMoveOnGlobe", func(t *testing.T) {
		vmc, err := newValidatedMotionCfg(nil, requestTypeMoveOnGlobe)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vmc, test.ShouldResemble, &validatedMotionConfiguration{
			angularDegsPerSec:     defaultAngularDegsPerSec,
			linearMPerSec:         defaultLinearMPerSec,
			obstaclePollingFreqHz: defaultObstaclePollingHz,
			positionPollingFreqHz: defaultPositionPollingHz,
			planDeviationMM:       defaultGlobePlanDeviationM * 1e3,
			obstacleDetectors:     []motion.ObstacleDetectorName{},
		})
	})

	t.Run("returns expected defaults when given zero cfg for requestTypeMoveOnGlobe", func(t *testing.T) {
		vmc, err := newValidatedMotionCfg(&motion.MotionConfiguration{}, requestTypeMoveOnGlobe)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vmc, test.ShouldResemble, &validatedMotionConfiguration{
			angularDegsPerSec:     defaultAngularDegsPerSec,
			linearMPerSec:         defaultLinearMPerSec,
			obstaclePollingFreqHz: defaultObstaclePollingHz,
			positionPollingFreqHz: defaultPositionPollingHz,
			planDeviationMM:       defaultGlobePlanDeviationM * 1e3,
			obstacleDetectors:     []motion.ObstacleDetectorName{},
		})
	})

	t.Run("returns expected defaults when given zero cfg for requestTypeMoveOnMap", func(t *testing.T) {
		vmc, err := newValidatedMotionCfg(&motion.MotionConfiguration{}, requestTypeMoveOnMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vmc, test.ShouldResemble, &validatedMotionConfiguration{
			angularDegsPerSec:     defaultAngularDegsPerSec,
			linearMPerSec:         defaultLinearMPerSec,
			obstaclePollingFreqHz: defaultObstaclePollingHz,
			positionPollingFreqHz: defaultPositionPollingHz,
			planDeviationMM:       defaultSlamPlanDeviationM * 1e3,
			obstacleDetectors:     []motion.ObstacleDetectorName{},
		})
	})

	t.Run("allows overriding defaults", func(t *testing.T) {
		vmc, err := newValidatedMotionCfg(&motion.MotionConfiguration{
			AngularDegsPerSec:     10.,
			LinearMPerSec:         20.,
			PlanDeviationMM:       30.,
			PositionPollingFreqHz: 40,
			ObstaclePollingFreqHz: 50.,
			ObstacleDetectors: []motion.ObstacleDetectorName{
				{
					VisionServiceName: vision.Named("fakeVision"),
					CameraName:        camera.Named("fakeCamera"),
				},
			},
		}, requestTypeMoveOnMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vmc, test.ShouldResemble, &validatedMotionConfiguration{
			angularDegsPerSec:     10.,
			linearMPerSec:         20.,
			planDeviationMM:       30.,
			positionPollingFreqHz: 40.,
			obstaclePollingFreqHz: 50.,
			obstacleDetectors: []motion.ObstacleDetectorName{
				{
					VisionServiceName: vision.Named("fakeVision"),
					CameraName:        camera.Named("fakeCamera"),
				},
			},
		})
	})
}

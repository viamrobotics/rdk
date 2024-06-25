package builtin

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

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
			kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -50}))
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
			logger.Debug(err.Error())
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
		logger.Debug(spatialmath.PoseToProtobuf(endPos.Pose()))
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
		test.That(t, err, test.ShouldBeError, motion.ErrGoalWithinPlanDeviation)
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

func TestMoveOnMapAskewIMU(t *testing.T) {
	t.Parallel()
	extraPosOnly := map[string]interface{}{"smooth_iter": 5, "motion_profile": "position_only"}
	t.Run("Askew but valid base should be able to plan", func(t *testing.T) {
		t.Parallel()
		logger := logging.NewTestLogger(t)
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
		logger.Debug(spatialmath.PoseToProtobuf(endPos))
		logger.Debug(spatialmath.PoseToProtobuf(goal1BaseFrame))

		test.That(t, spatialmath.PoseAlmostEqualEps(endPos, goal1BaseFrame, 10), test.ShouldBeTrue)
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
	injectSlam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, error) {
		return spatialmath.NewPose(
			r3.Vector{X: 0.58772e3, Y: -0.80826e3, Z: 0},
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 90},
		), nil
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

		// We place an obstacle on the right side of the robot to force our motion planner to return a path
		// which veers to the left. We then place an obstacle to the left of the robot and project the
		// robot's position across the path. By showing that we have a collision on the path with an
		// obstacle on the left we prove that our path does not collide with the original obstacle
		// placed on the right.
		obstacleLeft, err := spatialmath.NewBox(
			spatialmath.NewPose(r3.Vector{X: 0.22981e3, Y: -0.38875e3, Z: 0},
				&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 45}),
			r3.Vector{X: 900, Y: 10, Z: 10},
			"obstacleLeft",
		)
		test.That(t, err, test.ShouldBeNil)
		obstacleRight, err := spatialmath.NewBox(
			spatialmath.NewPose(r3.Vector{0.89627e3, -0.37192e3, 0},
				&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -45}),
			r3.Vector{900, 10, 10},
			"obstacleRight",
		)
		test.That(t, err, test.ShouldBeNil)

		req.Obstacles = []spatialmath.Geometry{obstacleRight}

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
		// collides with obstacleLeft
		wrldSt, err := referenceframe.NewWorldState(
			[]*referenceframe.GeometriesInFrame{
				referenceframe.NewGeometriesInFrame(
					referenceframe.World,
					[]spatialmath.Geometry{obstacleLeft},
				),
			}, nil,
		)
		test.That(t, err, test.ShouldBeNil)

		currentInputs := map[string][]referenceframe.Input{
			mr.kinematicBase.Kinematics().Name(): {
				{Value: 0}, // ptg index
				{Value: 0}, // trajectory alpha within ptg
				{Value: 0}, // start distance along trajectory index
				{Value: 0}, // end distace along trajectory index
			},
			mr.kinematicBase.LocalizationFrame().Name(): {
				{Value: 587.720000000000027284841053},  // X
				{Value: -808.259999999999990905052982}, // Y
				{Value: 0},                             // Z
				{Value: 0},                             // OX
				{Value: 0},                             // OY
				{Value: 1},                             // OZ
				{Value: 0},                             // Theta
			},
		}

		baseExecutionState, err := motionplan.NewExecutionState(
			plan, 1, currentInputs,
			map[string]*referenceframe.PoseInFrame{
				mr.kinematicBase.LocalizationFrame().Name(): referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPose(
					r3.Vector{X: 0.58772e3, Y: -0.80826e3, Z: 0},
					&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0},
				)),
			},
		)
		test.That(t, err, test.ShouldBeNil)

		augmentedBaseExecutionState, err := mr.augmentBaseExecutionState(baseExecutionState)
		test.That(t, err, test.ShouldBeNil)

		wrapperFrame := mr.localizaingFS.Frame(mr.kinematicBase.Name().Name)

		test.That(t, err, test.ShouldBeNil)
		err = motionplan.CheckPlan(
			wrapperFrame,
			augmentedBaseExecutionState,
			wrldSt,
			mr.localizaingFS,
			lookAheadDistanceMM,
			logger,
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, strings.Contains(err.Error(), "found constraint violation or collision in segment between"), test.ShouldBeTrue)
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

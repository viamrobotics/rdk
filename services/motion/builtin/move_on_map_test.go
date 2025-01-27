package builtin

import (
	"context"
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

const PlanDeviationMM = 150

func TestMoveOnMap(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	var pollingFreq float64
	motionCfg := &motion.MotionConfiguration{
		PositionPollingFreqHz: &pollingFreq,
		PlanDeviationMM:       100,
		LinearMPerSec:         0.3,
		AngularDegsPerSec:     60,
	}

	t.Run("Timeout", func(t *testing.T) {
		cfg, err := config.Read(ctx, "../data/real_wheeled_base.json", logger)
		test.That(t, err, test.ShouldBeNil)
		myRobot, err := robotimpl.New(ctx, cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, myRobot.Close(context.Background()), test.ShouldBeNil)
		}()

		injectSlam := createInjectedSlam("test_slam")

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
		_, ms, closeFunc := CreateMoveOnMapTestEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
		defer closeFunc(ctx)
		easyGoalInBaseFrame := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.277 * 1000, Y: 0.593 * 1000})
		easyGoalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, easyGoalInBaseFrame)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   easyGoalInSLAMFrame,
			MotionCfg:     motionCfg,
			SlamName:      slam.Named("test_slam"),
			Extra:         map[string]interface{}{"smooth_iter": 0},
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

		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: baseResource})
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
		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: baseResource})
		test.That(t, err, test.ShouldBeNil)
		ph3, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ph3, test.ShouldResemble, ph2)
	})

	t.Run("pass when within plan dev m of goal without position_only due to theta difference in goal", func(t *testing.T) {
		_, ms, closeFunc := CreateMoveOnMapTestEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40, nil)
		defer closeFunc(ctx)

		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			MotionCfg:     motionCfg,
			Destination:   spatialmath.NewPoseFromOrientation(&spatialmath.EulerAngles{Yaw: 3}),
			SlamName:      slam.Named("test_slam"),
		}

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*5)
		defer timeoutFn()
		executionID, err := ms.(*builtIn).MoveOnMap(timeoutCtx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotBeEmpty)
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
	injectSlam := createInjectedSlam(slamName)
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

		currentInputs := referenceframe.FrameSystemInputs{
			mr.kinematicBase.Kinematics().Name(): {
				{Value: 0}, // ptg index
				{Value: 0}, // trajectory alpha within ptg
				{Value: 0}, // start distance along trajectory index
				{Value: 0}, // end distace along trajectory index
			},
			mr.kinematicBase.LocalizationFrame().Name(): {
				{Value: 587},  // X
				{Value: -808}, // Y
				{Value: 0},    // Z
				{Value: 0},    // OX
				{Value: 0},    // OY
				{Value: 1},    // OZ
				{Value: 0},    // Theta
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

		wrapperFrame := mr.localizingFS.Frame(mr.kinematicBase.Name().Name)

		test.That(t, err, test.ShouldBeNil)
		err = motionplan.CheckPlan(
			wrapperFrame,
			augmentedBaseExecutionState,
			wrldSt,
			mr.localizingFS,
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

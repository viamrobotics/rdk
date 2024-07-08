package builtin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	viz "go.viam.com/rdk/vision"
)

func TestMoveOnGlobe(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	// Near antarctica 🐧
	gpsPoint := geo.NewPoint(-70, 40)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+7e-5)
	// create motion config
	extra := map[string]interface{}{
		"motion_profile": "position_only",
		"timeout":        15.,
		"smooth_iter":    5.,
	}

	t.Run("Changes to executions show up in PlanHistory", func(t *testing.T) {
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, gpsPoint, 80, nil)
		defer closeFunc(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
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

	t.Run("fail because of obstacle", func(t *testing.T) {
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, gpsPoint, 80, nil)
		defer closeFunc(ctx)

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
		geoGeometry := spatialmath.NewGeoGeometry(gpsPoint, []spatialmath.Geometry{geometry1, geometry2, geometry3, geometry4})

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			Destination:        dst,
			MovementSensorName: moveSensorResource,
			Obstacles:          []*spatialmath.GeoGeometry{geoGeometry},
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
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, gpsPoint, 80, nil)
		defer closeFunc(ctx)
		movementSensorInBase, err := ms.GetPose(ctx, resource.NewName(movementsensor.API, "test-gps"), "test-base", nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, movementSensorInBase.Pose().Point(), test.ShouldResemble, movementSensorInBasePoint)
	})
}

func TestBoundingRegionsConstraint(t *testing.T) {
	ctx := context.Background()
	origin := geo.NewPoint(0, 0)
	dst := geo.NewPoint(origin.Lat(), origin.Lng()+1e-5)
	extra := map[string]interface{}{
		"motion_profile": "position_only",
		"timeout":        5.,
		"smooth_iter":    5.,
	}
	motionCfg := &motion.MotionConfiguration{
		PlanDeviationMM: 10,
	}
	// Note: spatialmath.GeoPointToPoint(dst, origin) produces r3.Vector{1111.92, 0, 0}

	t.Run("starting in collision with bounding regions works", func(t *testing.T) {
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, origin, 80, nil)
		defer closeFunc(ctx)

		box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{2224, 2224, 2}, "")
		test.That(t, err, test.ShouldBeNil)

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Destination:        dst,
			BoundingRegions: []*spatialmath.GeoGeometry{
				spatialmath.NewGeoGeometry(origin, []spatialmath.Geometry{box}),
			},
			MotionCfg: motionCfg,
			Extra:     extra,
		}
		_, err = ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("starting outside of bounding regions fails", func(t *testing.T) {
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, origin, 80, nil)
		defer closeFunc(ctx)

		box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{2222, 2222, 2}, "")
		test.That(t, err, test.ShouldBeNil)

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Destination:        dst,
			BoundingRegions: []*spatialmath.GeoGeometry{
				spatialmath.NewGeoGeometry(geo.NewPoint(20, 20), []spatialmath.Geometry{box}),
			},
			MotionCfg: motionCfg,
			Extra:     extra,
		}
		_, err = ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldNotBeNil)
		expectedErrorString := "frame named test-base is not within the provided bounding regions"
		test.That(t, strings.Contains(err.Error(), expectedErrorString), test.ShouldBeTrue)
	})

	t.Run("implicit success with no bounding regions", func(t *testing.T) {
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, origin, 80, nil)
		defer closeFunc(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Destination:        dst,
			MotionCfg:          motionCfg,
			Extra:              extra,
		}
		_, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("fail to plan outside of bounding regions", func(t *testing.T) {
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, origin, 80, nil)
		defer closeFunc(ctx)

		box, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{500, 500, 2}, "")
		test.That(t, err, test.ShouldBeNil)

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Destination:        dst,
			MotionCfg:          motionCfg,
			BoundingRegions: []*spatialmath.GeoGeometry{
				spatialmath.NewGeoGeometry(geo.NewPoint(0, 0), []spatialmath.Geometry{box}),
			},
			Extra: extra,
		}
		_, err = ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, errors.New("destination was not within the provided bounding regions"))
	})

	t.Run("list of bounding regions - success case", func(t *testing.T) {
		// string together multiple bounding regions, such that the robot must
		// actually account for them in order to create a valid path
		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(ctx, t, origin, 80, nil)
		defer closeFunc(ctx)

		box1, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{500, 500, 2}, "")
		test.That(t, err, test.ShouldBeNil)
		box2, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{681, 0, 0}),
			r3.Vector{900, 900, 2}, "",
		)
		test.That(t, err, test.ShouldBeNil)

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Destination:        dst,
			MotionCfg:          motionCfg,
			BoundingRegions: []*spatialmath.GeoGeometry{
				spatialmath.NewGeoGeometry(geo.NewPoint(0, 0), []spatialmath.Geometry{box1}),
				spatialmath.NewGeoGeometry(geo.NewPoint(0, 0), []spatialmath.Geometry{box2}),
			},
			Extra: extra,
		}
		_, err = ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestObstacleReplanningGlobe(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()

	gpsOrigin := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsOrigin.Lat(), gpsOrigin.Lng()+1e-5)
	epsilonMM := 150.

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

	obstaclePollingFreq := 5.
	positionPollingFreq := 0.
	cfg := &motion.MotionConfiguration{
		PositionPollingFreqHz: &positionPollingFreq,
		ObstaclePollingFreqHz: &obstaclePollingFreq,
		PlanDeviationMM:       epsilonMM,
		ObstacleDetectors:     obstacleDetectorSlice,
		LinearMPerSec:         0.5,
		AngularDegsPerSec:     60,
	}

	extra := map[string]interface{}{"max_replans": 10, "max_ik_solutions": 1, "smooth_iter": 1, "motion_profile": "position_only"}
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
				// Note: for CreateMoveOnGlobeTestEnvironment, the camera is given an orientation such that it is pointing left, not
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
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: 900, Y: 0, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 1, Y: 1, Z: 10}, caseName)
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

		_, ms, closeFunc := CreateMoveOnGlobeTestEnvironment(
			ctx,
			t,
			gpsOrigin,
			1,
			nil,
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
			// These cannot be parallel or the `defer`s will cause failures
			testFn(t, c)
		})
	}
}

package builtin

import (
	"context"
	"math"
	"testing"
	"time"
	"fmt"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

func TestMoveOnGlobe(t *testing.T) {
	ctx := context.Background()
	ctx, cFunc := context.WithCancel(ctx)
	defer cFunc()
	// Near antarctica 🐧
	gpsPoint := geo.NewPoint(-70, 40)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+7e-5)
	expectedDst := r3.Vector{X: 2662.16, Y: 0, Z: 0} // Relative pose to the starting point of the base; facing north, Y = forwards
	epsilonMM := 15.
	// create motion config
	extra := map[string]interface{}{
		"motion_profile": "position_only",
		"timeout":        15.,
		"smooth_iter":    5.,
	}

	t.Run("Changes to executions show up in PlanHistory", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
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

	t.Run("is able to reach a nearby geo point with empty values", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested NaN heading", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Heading:            math.NaN(),
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested positive heading", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Heading:            10000000,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested negative heading", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Heading:            -10000000,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point when the motion configuration is empty", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
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
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Heading:            90,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("ensure success to a nearby geo point", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1}
		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			MovementSensorName: moveSensorResource,
			Destination:        dst,
			Obstacles:          []*spatialmath.GeoGeometry{},
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

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*15)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("go around an obstacle", func(t *testing.T) {
		localizer, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		planDeviationMM := 100.
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 0.0000001, LinearMPerSec: 0.2, AngularDegsPerSec: 60}

		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 50, Y: 0, Z: 0})
		boxDims := r3.Vector{X: 5, Y: 50, Z: 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoGeometry := spatialmath.NewGeoGeometry(gpsPoint, []spatialmath.Geometry{geometries})
		startPose, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		req := motion.MoveOnGlobeReq{
			ComponentName:      baseResource,
			Destination:        dst,
			MovementSensorName: moveSensorResource,
			Obstacles:          []*spatialmath.GeoGeometry{geoGeometry},
			MotionCfg:          motionCfg,
			Extra:              extra,
		}

		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*25)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)

		endPose, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		movedPose := spatialmath.PoseBetween(startPose.Pose(), endPose.Pose())
		fmt.Println("movedPose", spatialmath.PoseToProtobuf(movedPose))
		test.That(t, movedPose.Point().X, test.ShouldAlmostEqual, expectedDst.X, planDeviationMM)
		test.That(t, movedPose.Point().Y, test.ShouldAlmostEqual, expectedDst.Y, planDeviationMM)
	})

	t.Run("fail because of obstacle", func(t *testing.T) {
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
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
		_, ms, closeFunc := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer closeFunc(ctx)
		movementSensorInBase, err := ms.GetPose(ctx, resource.NewName(movementsensor.API, "test-gps"), "test-base", nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, movementSensorInBase.Pose().Point(), test.ShouldResemble, movementSensorInBasePoint)
	})
}

package builtin

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
)

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

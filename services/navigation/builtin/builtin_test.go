package builtin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/base"
	fakebase "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/base/kinematicbase"
	_ "go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/slam"
	fakeslam "go.viam.com/rdk/services/slam/fake"
	_ "go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/colordetector"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func setupNavigationServiceFromConfig(t *testing.T, configFilename string) (navigation.Service, func()) {
	t.Helper()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	svc, err := navigation.FromRobot(myRobot, "test_navigation")
	test.That(t, err, test.ShouldBeNil)
	return svc, func() {
		myRobot.Close(context.Background())
	}
}

func currentInputsShouldEqual(ctx context.Context, t *testing.T, kinematicBase kinematicbase.KinematicBase, pt *geo.Point) {
	t.Helper()
	inputs, err := kinematicBase.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	actualPt := geo.NewPoint(inputs[0].Value, inputs[1].Value)
	test.That(t, actualPt.Lat(), test.ShouldEqual, pt.Lat())
	test.That(t, actualPt.Lng(), test.ShouldEqual, pt.Lng())
}

func blockTillCallCount(t *testing.T, callCount int, callChan chan struct{}, timeout time.Duration) {
	t.Helper()
	waitForCallsTimeOutCtx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()
	for i := 0; i < callCount; i++ {
		select {
		case <-callChan:
		case <-waitForCallsTimeOutCtx.Done():
			t.Log("timed out waiting for test to finish")
			t.FailNow()
		}
	}
}

func deleteAllWaypoints(ctx context.Context, svc navigation.Service) error {
	waypoints, err := svc.(*builtIn).store.Waypoints(ctx)
	if err != nil {
		return err
	}
	for _, wp := range waypoints {
		if err := svc.RemoveWaypoint(ctx, wp.ID, nil); err != nil {
			return err
		}
	}
	return nil
}

func TestSetMode(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		description string
		mode        navigation.Mode
		expectedErr error
	}{
		{
			description: "setting mode to manual",
			mode:        navigation.ModeManual,
			expectedErr: nil,
		},
		{
			description: "setting mode to waypoint",
			mode:        navigation.ModeWaypoint,
			expectedErr: nil,
		},
		{
			description: "setting mode to explore",
			mode:        navigation.ModeExplore,
			expectedErr: errors.New("navigation mode 'explore' is not currently available"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			ns, teardown := setupNavigationServiceFromConfig(t, "../data/nav_cfg.json")
			defer teardown()

			navMode, err := ns.Mode(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, navMode, test.ShouldEqual, navigation.ModeManual)

			// Since navigation starts in manual mode, we need to set it to another mode first in order to test
			if tt.mode == navigation.ModeManual {
				err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
				test.That(t, err, test.ShouldBeNil)
			}

			err = ns.SetMode(ctx, tt.mode, nil)
			if tt.expectedErr == nil {
				test.That(t, err, test.ShouldEqual, tt.expectedErr)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldEqual, tt.expectedErr.Error())
			}
		})
	}
}

func TestNavSetup(t *testing.T) {
	ns, teardown := setupNavigationServiceFromConfig(t, "../data/nav_cfg.json")
	defer teardown()
	ctx := context.Background()

	navMode, err := ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, navigation.ModeManual)

	err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
	test.That(t, err, test.ShouldBeNil)
	navMode, err = ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, navigation.ModeWaypoint)

	// Prevent race
	err = ns.SetMode(ctx, navigation.ModeManual, nil)
	test.That(t, err, test.ShouldBeNil)

	geoPose, err := ns.Location(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	expectedGeoPose := spatialmath.NewGeoPose(geo.NewPoint(40.7, -73.98), 25.)
	test.That(t, geoPose, test.ShouldResemble, expectedGeoPose)

	wayPt, err := ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, wayPt, test.ShouldBeEmpty)

	pt := geo.NewPoint(0, 0)
	err = ns.AddWaypoint(ctx, pt, nil)
	test.That(t, err, test.ShouldBeNil)

	wayPt, err = ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(wayPt), test.ShouldEqual, 1)

	id := wayPt[0].ID
	err = ns.RemoveWaypoint(ctx, id, nil)
	test.That(t, err, test.ShouldBeNil)
	wayPt, err = ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(wayPt), test.ShouldEqual, 0)

	// Calling RemoveWaypoint on an already removed waypoint doesn't return an error
	err = ns.RemoveWaypoint(ctx, id, nil)
	test.That(t, err, test.ShouldBeNil)

	obs, err := ns.GetObstacles(ctx, nil)
	test.That(t, len(obs), test.ShouldEqual, 1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(ns.(*builtIn).motionCfg.VisionServices), test.ShouldEqual, 1)

	_, err = ns.GetPaths(ctx, nil)
	test.That(t, err.Error(), test.ShouldEqual, "unimplemented")
}

func TestStartWaypoint(t *testing.T) {
	// TODO(RSDK-5193): unskip this test
	t.Skip()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	injectMS := inject.NewMotionService("test_motion")
	cfg := resource.Config{
		Name:  "test_base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 100}},
	}

	fakeBase, err := fakebase.NewBase(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	fakeSlam := fakeslam.NewSLAM(slam.Named("foo"), logger)
	limits, err := fakeSlam.Limits(ctx)
	test.That(t, err, test.ShouldBeNil)

	localizer := motion.NewSLAMLocalizer(fakeSlam)
	test.That(t, err, test.ShouldBeNil)

	// cast fakeBase
	fake, ok := fakeBase.(*fakebase.Base)
	test.That(t, ok, test.ShouldBeTrue)

	options := kinematicbase.NewKinematicBaseOptions()
	options.PositionOnlyMode = false

	// TODO: Test this with PTGs
	kinematicBase, err := kinematicbase.WrapWithFakeDiffDriveKinematics(ctx, fake, localizer, limits, options, nil)
	test.That(t, err, test.ShouldBeNil)

	injectMovementSensor := inject.NewMovementSensor("test_movement")
	injectMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		inputs, err := kinematicBase.CurrentInputs(ctx)
		return geo.NewPoint(inputs[0].Value, inputs[1].Value), 0, err
	}

	ns, err := NewBuiltIn(
		ctx,
		resource.Dependencies{injectMS.Name(): injectMS, fakeBase.Name(): fakeBase, injectMovementSensor.Name(): injectMovementSensor},
		resource.Config{
			ConvertedAttributes: &Config{
				Store: navigation.StoreConfig{
					Type: navigation.StoreTypeMemory,
				},
				BaseName:           "test_base",
				MovementSensorName: "test_movement",
				MotionServiceName:  "test_motion",
				DegPerSec:          1,
				MetersPerSec:       1,
			},
		},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, ns.Close(context.Background()), test.ShouldBeNil)
	}()

	t.Run("Reach waypoints successfully", func(t *testing.T) {
		callChan := make(chan struct{}, 2)
		injectMS.MoveOnGlobeFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *geo.Point,
			heading float64,
			movementSensorName resource.Name,
			obstacles []*spatialmath.GeoObstacle,
			motionCfg *motion.MotionConfiguration,
			extra map[string]interface{},
		) (bool, error) {
			err := kinematicBase.GoToInputs(ctx, referenceframe.FloatsToInputs([]float64{destination.Lat(), destination.Lng()}))
			callChan <- struct{}{}
			return true, err
		}
		pt := geo.NewPoint(1, 0)
		err = ns.AddWaypoint(ctx, pt, nil)
		test.That(t, err, test.ShouldBeNil)

		pt = geo.NewPoint(3, 1)
		err = ns.AddWaypoint(ctx, pt, nil)
		test.That(t, err, test.ShouldBeNil)

		ns.(*builtIn).mode = navigation.ModeManual
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)
		blockTillCallCount(t, 2, callChan, time.Second*5)
		ns.(*builtIn).wholeServiceCancelFunc()
		ns.(*builtIn).activeBackgroundWorkers.Wait()

		currentInputsShouldEqual(ctx, t, kinematicBase, pt)
	})

	t.Run("Extra defaults to motion_profile", func(t *testing.T) {
		callChan := make(chan struct{}, 1)
		// setup injected MoveOnGlobe to test what extra defaults to from startWaypointExperimental function
		injectMS.MoveOnGlobeFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *geo.Point,
			heading float64,
			movementSensorName resource.Name,
			obstacles []*spatialmath.GeoObstacle,
			motionCfg *motion.MotionConfiguration,
			extra map[string]interface{},
		) (bool, error) {
			callChan <- struct{}{}
			if extra != nil && extra["motion_profile"] != nil {
				return true, nil
			}
			return false, errors.New("no motion_profile exist")
		}

		// construct new point to navigate to
		pt := geo.NewPoint(0, 0)
		err = ns.AddWaypoint(ctx, pt, nil)
		test.That(t, err, test.ShouldBeNil)

		cancelCtx, fn := context.WithCancel(ctx)
		ns.(*builtIn).startWaypointMode(cancelCtx, map[string]interface{}{})
		blockTillCallCount(t, 1, callChan, time.Second*5)
		fn()
		ns.(*builtIn).activeBackgroundWorkers.Wait()

		// go to same point again
		err = ns.AddWaypoint(ctx, pt, nil)
		test.That(t, err, test.ShouldBeNil)

		cancelCtx, fn = context.WithCancel(ctx)
		ns.(*builtIn).startWaypointMode(cancelCtx, nil)
		blockTillCallCount(t, 1, callChan, time.Second*5)
		fn()
		ns.(*builtIn).activeBackgroundWorkers.Wait()
	})

	t.Run("Test MoveOnGlobe cancellation and errors", func(t *testing.T) {
		eventChannel, statusChannel := make(chan string), make(chan string, 1)
		cancelledContextMsg := "context cancelled"
		hitAnErrorMsg := "hit an error"
		arrivedAtWaypointMsg := "arrived at destination"
		invalidStateMsg := "bad message passed to event channel"

		injectMS.MoveOnGlobeFunc = func(
			ctx context.Context,
			componentName resource.Name,
			destination *geo.Point,
			heading float64,
			movementSensorName resource.Name,
			obstacles []*spatialmath.GeoObstacle,
			motionCfg *motion.MotionConfiguration,
			extra map[string]interface{},
		) (bool, error) {
			if ctx.Err() != nil {
				statusChannel <- cancelledContextMsg
				return false, ctx.Err()
			}
			select {
			case <-ctx.Done():
				statusChannel <- cancelledContextMsg
				return false, ctx.Err()
			case msg := <-eventChannel:
				var err error
				if msg == arrivedAtWaypointMsg {
					err = kinematicBase.GoToInputs(
						ctx,
						referenceframe.FloatsToInputs([]float64{destination.Lat(), destination.Lng()}),
					)
				}

				statusChannel <- msg
				switch {
				case msg == hitAnErrorMsg:
					return false, errors.New(hitAnErrorMsg)
				case msg == arrivedAtWaypointMsg:
					return true, err
				default:
					// should be unreachable
					return false, errors.New(invalidStateMsg)
				}
			}
		}

		pt1, pt2, pt3 := geo.NewPoint(1, 2), geo.NewPoint(2, 3), geo.NewPoint(3, 4)
		points := []*geo.Point{pt1, pt2, pt3}
		t.Run("MoveOnGlobe error results in skipping the current waypoint", func(t *testing.T) {
			// Set manual mode to ensure waypoint loop from prior test exits
			err = ns.SetMode(ctx, navigation.ModeManual, map[string]interface{}{"experimental": true})
			test.That(t, err, test.ShouldBeNil)
			ctx, cancelFunc := context.WithCancel(ctx)
			defer ns.(*builtIn).activeBackgroundWorkers.Wait()
			defer cancelFunc()
			err = deleteAllWaypoints(ctx, ns)
			for _, pt := range points {
				err = ns.AddWaypoint(ctx, pt, nil)
				test.That(t, err, test.ShouldBeNil)
			}

			ns.(*builtIn).startWaypointMode(ctx, map[string]interface{}{"experimental": true})

			// Get the ID of the first waypoint
			wp1, err := ns.(*builtIn).store.NextWaypoint(ctx)
			test.That(t, err, test.ShouldBeNil)

			// Reach the first waypoint
			eventChannel <- arrivedAtWaypointMsg
			test.That(t, <-statusChannel, test.ShouldEqual, arrivedAtWaypointMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt1)

			// Ensure we aren't querying before the nav service has a chance to mark the previous waypoint visited.
			wp2, err := ns.(*builtIn).store.NextWaypoint(ctx)
			test.That(t, err, test.ShouldBeNil)
			for wp2.ID == wp1.ID {
				wp2, err = ns.(*builtIn).store.NextWaypoint(ctx)
				test.That(t, err, test.ShouldBeNil)
			}

			// Skip the second waypoint due to an error
			eventChannel <- hitAnErrorMsg
			test.That(t, <-statusChannel, test.ShouldEqual, hitAnErrorMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt1)

			// Ensure we aren't querying before the nav service has a chance to mark the previous waypoint visited.
			wp3, err := ns.(*builtIn).store.NextWaypoint(ctx)
			test.That(t, err, test.ShouldBeNil)
			for wp3.ID == wp2.ID {
				wp3, err = ns.(*builtIn).store.NextWaypoint(ctx)
				test.That(t, err, test.ShouldBeNil)
			}

			// Reach the third waypoint
			eventChannel <- arrivedAtWaypointMsg
			test.That(t, <-statusChannel, test.ShouldEqual, arrivedAtWaypointMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt3)
		})
		t.Run("Calling SetMode cancels current and future MoveOnGlobe calls", func(t *testing.T) {
			// Set manual mode to ensure waypoint loop from prior test exits
			err = deleteAllWaypoints(ctx, ns)
			test.That(t, err, test.ShouldBeNil)
			for _, pt := range points {
				err = ns.AddWaypoint(ctx, pt, nil)
				test.That(t, err, test.ShouldBeNil)
			}

			// start navigation - set ModeManual first to ensure navigation starts up
			err = ns.SetMode(ctx, navigation.ModeWaypoint, map[string]interface{}{"experimental": true})

			// Reach the first waypoint
			eventChannel <- arrivedAtWaypointMsg
			test.That(t, <-statusChannel, test.ShouldEqual, arrivedAtWaypointMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt1)

			// Change the mode to manual --> stops navigation to waypoints
			err = ns.SetMode(ctx, navigation.ModeManual, map[string]interface{}{"experimental": true})
			test.That(t, err, test.ShouldBeNil)
			select {
			case msg := <-statusChannel:
				test.That(t, msg, test.ShouldEqual, cancelledContextMsg)
			case <-time.After(5 * time.Second):
				ns.(*builtIn).activeBackgroundWorkers.Wait()
			}
			currentInputsShouldEqual(ctx, t, kinematicBase, pt1)
		})

		t.Run("Calling RemoveWaypoint on the waypoint in progress cancels current MoveOnGlobe call", func(t *testing.T) {
			// Set manual mode to ensure waypoint loop from prior test exits
			err = ns.SetMode(ctx, navigation.ModeManual, map[string]interface{}{"experimental": true})
			test.That(t, err, test.ShouldBeNil)
			err = deleteAllWaypoints(ctx, ns)
			for _, pt := range points {
				err = ns.AddWaypoint(ctx, pt, nil)
				test.That(t, err, test.ShouldBeNil)
			}

			// start navigation - set ModeManual first to ensure navigation starts up
			err = ns.SetMode(ctx, navigation.ModeWaypoint, map[string]interface{}{"experimental": true})

			// Get the ID of the first waypoint
			wp1, err := ns.(*builtIn).store.NextWaypoint(ctx)
			test.That(t, err, test.ShouldBeNil)

			// Reach the first waypoint
			eventChannel <- arrivedAtWaypointMsg
			test.That(t, <-statusChannel, test.ShouldEqual, arrivedAtWaypointMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt1)

			// Remove the second waypoint, which is in progress. Ensure we aren't querying before the nav service has a chance to mark
			// the previous waypoint visited.
			wp2, err := ns.(*builtIn).store.NextWaypoint(ctx)
			test.That(t, err, test.ShouldBeNil)
			for wp2.ID == wp1.ID {
				wp2, err = ns.(*builtIn).store.NextWaypoint(ctx)
				test.That(t, err, test.ShouldBeNil)
			}

			// ensure we actually start the wp2 waypoint before removing it
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				ns.(*builtIn).mu.RLock()
				svcWp := ns.(*builtIn).waypointInProgress
				ns.(*builtIn).mu.RUnlock()
				test.That(tb, wp2.ID, test.ShouldEqual, svcWp.ID)
			})

			err = ns.RemoveWaypoint(ctx, wp2.ID, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, <-statusChannel, test.ShouldEqual, cancelledContextMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt1)

			// Reach the third waypoint
			eventChannel <- arrivedAtWaypointMsg
			test.That(t, <-statusChannel, test.ShouldEqual, arrivedAtWaypointMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt3)
		})

		t.Run("Calling RemoveWaypoint on a waypoint that is not in progress does not cancel MoveOnGlobe", func(t *testing.T) {
			// Set manual mode to ensure waypoint loop from prior test exits
			err = ns.SetMode(ctx, navigation.ModeManual, map[string]interface{}{"experimental": true})
			test.That(t, err, test.ShouldBeNil)
			err = deleteAllWaypoints(ctx, ns)
			var wp3 navigation.Waypoint
			for i, pt := range points {
				if i < 3 {
					err = ns.AddWaypoint(ctx, pt, nil)
					test.That(t, err, test.ShouldBeNil)
				} else {
					wp3, err = ns.(*builtIn).store.AddWaypoint(ctx, pt)
					test.That(t, err, test.ShouldBeNil)
				}
			}

			// start navigation - set ModeManual first to ensure navigation starts up
			err = ns.SetMode(ctx, navigation.ModeWaypoint, map[string]interface{}{"experimental": true})

			// Reach the first waypoint
			eventChannel <- arrivedAtWaypointMsg
			test.That(t, <-statusChannel, test.ShouldEqual, arrivedAtWaypointMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt1)

			// Remove the third waypoint, which is not in progress yet
			err = ns.RemoveWaypoint(ctx, wp3.ID, nil)
			test.That(t, err, test.ShouldBeNil)

			// Reach the second waypoint
			eventChannel <- arrivedAtWaypointMsg
			test.That(t, <-statusChannel, test.ShouldEqual, arrivedAtWaypointMsg)
			currentInputsShouldEqual(ctx, t, kinematicBase, pt2)
		})
	})
}

func TestValidateGeometry(t *testing.T) {
	cfg := Config{
		BaseName:           "base",
		MovementSensorName: "localizer",
	}

	createBox := func(translation r3.Vector) Config {
		boxPose := spatialmath.NewPoseFromPoint(translation)
		geometries, err := spatialmath.NewBox(boxPose, r3.Vector{X: 10, Y: 10, Z: 10}, "")
		test.That(t, err, test.ShouldBeNil)

		geoObstacle := spatialmath.NewGeoObstacle(geo.NewPoint(0, 0), []spatialmath.Geometry{geometries})
		geoObstacleCfg, err := spatialmath.NewGeoObstacleConfig(geoObstacle)
		test.That(t, err, test.ShouldBeNil)

		cfg.Obstacles = []*spatialmath.GeoObstacleConfig{geoObstacleCfg}

		return cfg
	}

	t.Run("fail case", func(t *testing.T) {
		cfg = createBox(r3.Vector{X: 10, Y: 10, Z: 10})
		_, err := cfg.Validate("")
		expectedErr := "geometries specified through the navigation are not allowed to have a translation"
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, expectedErr)
	})

	t.Run("success case", func(t *testing.T) {
		cfg = createBox(r3.Vector{})
		_, err := cfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
	})
}

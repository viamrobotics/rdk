package builtin

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	fakebase "go.viam.com/rdk/components/base/fake"
	_ "go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
	_ "go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/colordetector"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

type moveOnGlobeReq struct {
	componentName      resource.Name
	destination        *geo.Point
	heading            float64
	movementSensorName resource.Name
	obstacles          []*spatialmath.GeoObstacle
	motionCfg          *motion.MotionConfiguration
	extra              map[string]interface{}
}

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
		err := myRobot.Close(context.Background())
		test.That(t, err, test.ShouldBeNil)
	}
}

func setupNavService(
	ctx context.Context,
	t *testing.T,
	injectMS *inject.MotionService,
	injectMSensor *inject.MovementSensor,
	fakeBase base.Base,
	logger golog.Logger,
) (navigation.Service, func()) {
	ns, err := NewBuiltIn(
		ctx,
		resource.Dependencies{injectMS.Name(): injectMS, fakeBase.Name(): fakeBase, injectMSensor.Name(): injectMSensor},
		resource.Config{
			ConvertedAttributes: &Config{
				Store: navigation.StoreConfig{
					Type: navigation.StoreTypeMemory,
				},
				BaseName:           fakeBase.Name().Name,
				MovementSensorName: injectMSensor.Name().Name,
				MotionServiceName:  injectMS.Name().Name,
				DegPerSec:          1,
				MetersPerSec:       1,
			},
		},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)
	return ns, func() {
		test.That(t, ns.Close(context.Background()), test.ShouldBeNil)
	}
}

func blockTillCalledOrTimeout(ctx context.Context, t *testing.T, expectedCall moveOnGlobeReq, callChan chan moveOnGlobeReq) {
	t.Helper()
	select {
	case c := <-callChan:
		test.That(t, c.componentName, test.ShouldResemble, expectedCall.componentName)
		test.That(t, c.destination, test.ShouldResemble, expectedCall.destination)
		test.That(t, c.extra, test.ShouldResemble, expectedCall.extra)
		// NaN doesn't equal itself so we need this if statement
		if math.IsNaN(expectedCall.heading) {
			test.That(t, math.IsNaN(c.heading), test.ShouldBeTrue)
		} else {
			test.That(t, c.heading, test.ShouldResemble, expectedCall.heading)
		}
		test.That(t, math.IsNaN(c.heading), test.ShouldBeTrue)
		test.That(t, c.motionCfg, test.ShouldResemble, expectedCall.motionCfg)
		test.That(t, c.movementSensorName, test.ShouldResemble, expectedCall.movementSensorName)
		test.That(t, c.obstacles, test.ShouldResemble, expectedCall.obstacles)
	case <-ctx.Done():
		t.Error("timed out waiting for test to finish")
		t.FailNow()
	}
}

func fakeBase(ctx context.Context, t *testing.T, logger golog.Logger) base.Base {
	cfg := resource.Config{
		Name:  "test_base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 100}},
	}

	fb, err := fakebase.NewBase(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	return fb
}

func pollTillWaypointLen(ctx context.Context, t *testing.T, ns navigation.Service, expectedLen int) ([]navigation.Waypoint, error) {
	t.Helper()
	timer := time.NewTimer(time.Millisecond * 50)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Error("pollTillWaypointLen timeout reached")
			return nil, ctx.Err()
		case <-timer.C:
			wps, err := ns.Waypoints(ctx, nil)
			if err != nil {
				return nil, err
			}

			if len(wps) == expectedLen {
				return wps, nil
			}
		}
	}
}

func wpSimilar(t *testing.T, wp1, wp2 navigation.Waypoint) {
	t.Helper()
	test.That(t, wp1.Visited, test.ShouldEqual, wp2.Visited)
	test.That(t, wp1.Lat, test.ShouldEqual, wp2.Lat)
	test.That(t, wp1.Long, test.ShouldEqual, wp2.Long)
}

func pointToWP(pt *geo.Point) navigation.Waypoint {
	return navigation.Waypoint{Lat: pt.Lat(), Long: pt.Lng(), Visited: false}
}

func TestNavSetup(t *testing.T) {
	ns, teardown := setupNavigationServiceFromConfig(t, "../data/nav_cfg.json")
	defer teardown()
	ctx := context.Background()

	// Mode defaults to manual mode
	navMode, err := ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, navigation.ModeManual)

	// Mode is able to be changed to Waypoint mode
	err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
	test.That(t, err, test.ShouldBeNil)
	navMode, err = ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, navigation.ModeWaypoint)

	// Set Mode back to default
	err = ns.SetMode(ctx, navigation.ModeManual, nil)
	test.That(t, err, test.ShouldBeNil)
	navMode, err = ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, navigation.ModeManual)

	// Mocked nav service location is a specific geo coordinate
	geoPose, err := ns.Location(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	expectedGeoPose := spatialmath.NewGeoPose(geo.NewPoint(40.7, -73.98), 25.)
	test.That(t, geoPose, test.ShouldResemble, expectedGeoPose)

	// Waypoints default to an empty slice
	wayPt, err := ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, wayPt, test.ShouldBeEmpty)

	// Waypoints are able to be added
	pt := geo.NewPoint(0, 0)
	err = ns.AddWaypoint(ctx, pt, nil)
	test.That(t, err, test.ShouldBeNil)

	// Added Waypoints are returned from Waypoints
	wayPt, err = ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(wayPt), test.ShouldEqual, 1)

	// Added Waypoints can be removed
	id := wayPt[0].ID
	err = ns.RemoveWaypoint(ctx, id, nil)
	test.That(t, err, test.ShouldBeNil)

	// Removed Waypoints are no longer returned from Waypoints
	wayPt, err = ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(wayPt), test.ShouldEqual, 0)

	// Calling RemoveWaypoint on an already removed waypoint doesn't return an error
	err = ns.RemoveWaypoint(ctx, id, nil)
	test.That(t, err, test.ShouldBeNil)

	// The mocked Nav service detects a single obstacle
	obs, err := ns.GetObstacles(ctx, nil)
	test.That(t, len(obs), test.ShouldEqual, 1)
	test.That(t, err, test.ShouldBeNil)

	// The motion config provides a single vision service
	test.That(t, len(ns.(*builtIn).motionCfg.VisionServices), test.ShouldEqual, 1)
}

func TestStartWaypoint(t *testing.T) {
	t.Run("Calls motion.MoveOnGlobe for every waypoint", func(t *testing.T) {
		callChan := make(chan moveOnGlobeReq)
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")
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
			callChan <- moveOnGlobeReq{
				componentName:      componentName,
				destination:        destination,
				heading:            heading,
				movementSensorName: movementSensorName,
				obstacles:          obstacles,
				motionCfg:          motionCfg,
				extra:              extra,
			}
			return true, nil
		}
		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// add waypoint 2
		pt2 := geo.NewPoint(3, 1)
		err = ns.AddWaypoint(ctx, pt2, nil)
		test.That(t, err, test.ShouldBeNil)

		// test that added waypoints are returned by the waypoints function
		wps, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps), test.ShouldEqual, 2)
		wpSimilar(t, wps[0], pointToWP(pt1))
		wpSimilar(t, wps[1], pointToWP(pt2))

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		// confirm mode is now Waypoint
		mode, err := ns.Mode(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, navigation.ModeWaypoint)

		expectedMotionCfg := &motion.MotionConfiguration{
			LinearMPerSec:     1,
			AngularDegsPerSec: 1,
		}

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()

		// confirm MoveOnGlobe is called for waypoint 1
		expectedCall1 := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt1,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg:          expectedMotionCfg,
			// TODO: fix with RSDK-4583
			// extra defaults to "motion_profile": "position_only"
			// extra: map[string]interface{}{"motion_profile": "position_only"},
		}
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall1, callChan)

		// confirm waypoint 1 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 1
		wps1, err := pollTillWaypointLen(timeoutCtx, t, ns, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps1), test.ShouldEqual, 1)
		wpSimilar(t, wps1[0], pointToWP(pt2))

		// confirm MoveOnGlobe is called for waypoint 2
		expectedCall2 := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt2,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg:          expectedMotionCfg,
			// TODO: fix with RSDK-4583
			// extra defaults to "motion_profile": "position_only"
			// extra: map[string]interface{}{"motion_profile": "position_only"},
		}
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall2, callChan)

		// confirm waypoint 2 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 2
		// This leaves the waypoint store empty
		wps2, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, wps2, test.ShouldBeEmpty)
	})

	t.Run("SetMode's extra[motion_profile] defaults to position_mode when extra is non nil & motion_profile is unset", func(t *testing.T) {
		// TODO: fix with RSDK-4583
		t.Skip()
		callChan := make(chan moveOnGlobeReq)
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")
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
			callChan <- moveOnGlobeReq{
				componentName:      componentName,
				destination:        destination,
				heading:            heading,
				movementSensorName: movementSensorName,
				obstacles:          obstacles,
				motionCfg:          motionCfg,
				extra:              extra,
			}
			return true, nil
		}
		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// test that added waypoints are returned by the waypoints function
		wps, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps), test.ShouldEqual, 1)
		wpSimilar(t, wps[0], pointToWP(pt1))

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		// confirm mode is now Waypoint
		mode, err := ns.Mode(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, navigation.ModeWaypoint)

		expectedMotionCfg := &motion.MotionConfiguration{
			LinearMPerSec:     1,
			AngularDegsPerSec: 1,
		}

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()

		// confirm MoveOnGlobe is called for waypoint 1
		expectedCall1 := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt1,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg:          expectedMotionCfg,
			// extra keeps default of "motion_profile": "position_only"
			extra: map[string]interface{}{"motion_profile": "position_only"},
		}
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall1, callChan)

		// confirm waypoint 1 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 1
		wps1, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps1), test.ShouldEqual, 0)
	})

	t.Run("SetMode's extra[motion_profile] is able to be overridden to something other than position_mode", func(t *testing.T) {
		callChan := make(chan moveOnGlobeReq)
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")
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
			callChan <- moveOnGlobeReq{
				componentName:      componentName,
				destination:        destination,
				heading:            heading,
				movementSensorName: movementSensorName,
				obstacles:          obstacles,
				motionCfg:          motionCfg,
				extra:              extra,
			}
			return true, nil
		}
		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, map[string]interface{}{"motion_profile": "SOMETHING ELSE"})
		test.That(t, err, test.ShouldBeNil)

		expectedMotionCfg := &motion.MotionConfiguration{
			LinearMPerSec:     1,
			AngularDegsPerSec: 1,
		}

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFn()

		// confirm MoveOnGlobe is called for waypoint 1
		expectedCall1 := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt1,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg:          expectedMotionCfg,
			extra:              map[string]interface{}{"motion_profile": "SOMETHING ELSE"},
		}
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall1, callChan)

		// confirm waypoint 1 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 1
		wps1, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, wps1, test.ShouldBeEmpty)
	})

	t.Run("MoveOnGlobe error results in retrying the same waypoint until success", func(t *testing.T) {
		callChan := make(chan moveOnGlobeReq)
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")

		called := false
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
			callChan <- moveOnGlobeReq{
				componentName:      componentName,
				destination:        destination,
				heading:            heading,
				movementSensorName: movementSensorName,
				obstacles:          obstacles,
				motionCfg:          motionCfg,
				extra:              extra,
			}
			if called {
				return true, nil
			}
			called = true
			return false, errors.New("move on globe error")
		}
		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// test that added waypoints are returned by the waypoints function
		wps, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps), test.ShouldEqual, 1)
		wpSimilar(t, wps[0], pointToWP(pt1))

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		expectedMotionCfg := &motion.MotionConfiguration{
			LinearMPerSec:     1,
			AngularDegsPerSec: 1,
		}

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()

		// confirm MoveOnGlobe is called for waypoint 1
		expectedCall1and2 := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt1,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg:          expectedMotionCfg,
			// TODO: fix with RSDK-4583
			// extra defaults to "motion_profile": "position_only"
			// extra: map[string]interface{}{"motion_profile": "position_only"},
		}
		// MoveOnGlobe called twice with same parameters, as nav services
		// retries first call which returned an error
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall1and2, callChan)
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall1and2, callChan)

		// confirm waypoint 1 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 1
		wps1, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps1), test.ShouldEqual, 0)
	})

	t.Run("MoveOnGlobe success false results in retrying the same waypoint until success", func(t *testing.T) {
		callChan := make(chan moveOnGlobeReq)
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")

		called := false
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
			callChan <- moveOnGlobeReq{
				componentName:      componentName,
				destination:        destination,
				heading:            heading,
				movementSensorName: movementSensorName,
				obstacles:          obstacles,
				motionCfg:          motionCfg,
				extra:              extra,
			}
			if called {
				return true, nil
			}
			called = true
			return false, nil
		}
		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// test that added waypoints are returned by the waypoints function
		wps, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps), test.ShouldEqual, 1)
		wpSimilar(t, wps[0], pointToWP(pt1))

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		expectedMotionCfg := &motion.MotionConfiguration{
			LinearMPerSec:     1,
			AngularDegsPerSec: 1,
		}

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()

		// confirm MoveOnGlobe is called for waypoint 1
		expectedCall1and2 := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt1,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg:          expectedMotionCfg,
			// TODO: fix with RSDK-4583
			// extra defaults to "motion_profile": "position_only"
			// extra: map[string]interface{}{"motion_profile": "position_only"},
		}
		// MoveOnGlobe called twice with same parameters, as nav services
		// retries first call which returned success false
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall1and2, callChan)
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall1and2, callChan)

		// confirm waypoint 1 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 1
		wps1, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps1), test.ShouldEqual, 0)
	})

	t.Run("Calling RemoveWaypoint on the waypoint in progress cancels current MoveOnGlobe call", func(t *testing.T) {
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")

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
			// // always error to prove that the waypoint was removed due to RemoveWaypoint
			// // and not MoveOnGlobe succeeding
			return false, nil
		}
		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// test that added waypoints are returned by the waypoints function
		wps, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps), test.ShouldEqual, 1)
		wpSimilar(t, wps[0], pointToWP(pt1))

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()

		// remove the waypoint
		err = ns.RemoveWaypoint(ctx, wps[0].ID, nil)
		test.That(t, err, test.ShouldBeNil)

		// confirm waypoint 1 is eventually removed from the waypoints store
		wps1, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps1), test.ShouldEqual, 0)
	})

	t.Run("Calling SetMode Manual pauses MoveOnGlobe calls, setting it back to waypoint resumes", func(t *testing.T) {
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")

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
			return false, nil
		}

		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// test that added waypoints are returned by the waypoints function
		wps1, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps1), test.ShouldEqual, 1)
		wpSimilar(t, wps1[0], pointToWP(pt1))

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()

		// change mode to stop MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeManual, nil)
		test.That(t, err, test.ShouldBeNil)

		// confirm first waypoint is still there as MoveOnGlobe
		// hasn't returned success yet
		wps2, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, wps2, test.ShouldResemble, wps1)

		callChan := make(chan moveOnGlobeReq)
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
			callChan <- moveOnGlobeReq{
				componentName:      componentName,
				destination:        destination,
				heading:            heading,
				movementSensorName: movementSensorName,
				obstacles:          obstacles,
				motionCfg:          motionCfg,
				extra:              extra,
			}
			return true, nil
		}

		// change mode to restart MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		// confirm MoveOnGlobe is called for waypoint 1
		expectedCall := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt1,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg: &motion.MotionConfiguration{
				LinearMPerSec:     1,
				AngularDegsPerSec: 1,
			},
			// TODO: fix with RSDK-4583
			// extra defaults to "motion_profile": "position_only"
			// extra: map[string]interface{}{"motion_profile": "position_only"},
		}
		// confirm that MoveOnGlobe is called with waypoint 1 after
		// switching from WaypointMode -> ManualMode -> WaypointMode
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall, callChan)

		// confirm waypoint 1 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 1
		wps3, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps3), test.ShouldEqual, 0)
	})

	t.Run("Calling RemoveWaypoint on a waypoint that is not in progress does not cancel MoveOnGlobe", func(t *testing.T) {
		injectMSensor := inject.NewMovementSensor("test_movement")

		injectMS := inject.NewMotionService("test_motion")
		var successAtomic atomic.Bool

		callChan := make(chan moveOnGlobeReq)
		successChan := make(chan struct{})
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
			callChan <- moveOnGlobeReq{
				componentName:      componentName,
				destination:        destination,
				heading:            heading,
				movementSensorName: movementSensorName,
				obstacles:          obstacles,
				motionCfg:          motionCfg,
				extra:              extra,
			}
			success := successAtomic.Load()
			if success {
				successChan <- struct{}{}
			}

			return success, nil
		}

		ctx := context.Background()
		logger := golog.NewTestLogger(t)
		b := fakeBase(ctx, t, logger)
		ns, teardown := setupNavService(ctx, t, injectMS, injectMSensor, b, logger)
		defer teardown()

		// add waypoint 1
		pt1 := geo.NewPoint(1, 0)
		err := ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		// add waypoint 2
		pt2 := geo.NewPoint(3, 1)
		err = ns.AddWaypoint(ctx, pt2, nil)
		test.That(t, err, test.ShouldBeNil)

		// test that added waypoints are returned by the waypoints function
		wps1, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps1), test.ShouldEqual, 2)
		wpSimilar(t, wps1[0], pointToWP(pt1))
		wpSimilar(t, wps1[1], pointToWP(pt2))

		// set mode to Waypoint to begin MoveOnMap calls
		err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		timeoutCtx, cancelFn := context.WithTimeout(context.Background(), time.Second)
		defer cancelFn()

		// confirm MoveOnGlobe is called for waypoint 1
		expectedCall := moveOnGlobeReq{
			componentName:      b.Name(),
			destination:        pt1,
			heading:            math.NaN(),
			movementSensorName: injectMSensor.Name(),
			motionCfg: &motion.MotionConfiguration{
				LinearMPerSec:     1,
				AngularDegsPerSec: 1,
			},
			// TODO: fix with RSDK-4583
			// extra defaults to "motion_profile": "position_only"
			// extra: map[string]interface{}{"motion_profile": "position_only"},
		}

		// delete last waypoint waypoint
		err = ns.RemoveWaypoint(ctx, wps1[1].ID, nil)
		test.That(t, err, test.ShouldBeNil)

		// confirm last waypoint was removed
		wps2, err := ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps2), test.ShouldEqual, 1)
		wpSimilar(t, wps2[0], pointToWP(pt1))

		// unblock the MoveOnGlobe request after removing waypoint 2
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall, callChan)

		// set success to true after waypoint 2 has been removed
		successAtomic.Store(true)

		// confirm that MoveOnGlobe is called with waypoint 1 after removing waypoint 2 waypoint
		blockTillCalledOrTimeout(timeoutCtx, t, expectedCall, callChan)

		// confirm success has occurred within the timeout
		test.That(t, utils.SelectContextOrWaitChan(timeoutCtx, successChan), test.ShouldBeTrue)

		// confirm waypoint 1 is eventually removed from the waypoints store after
		// MoveOnGlobe reports reaching waypoint 1
		wps3, err := pollTillWaypointLen(timeoutCtx, t, ns, 0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps3), test.ShouldEqual, 0)
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

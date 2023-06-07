// Package builtin contains the default navigation service, along with a gRPC server and client
package builtin

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/base"
	_ "go.viam.com/rdk/components/base/fake"
	fakebase "go.viam.com/rdk/components/base/fake"
	_ "go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/slam"
	fakeslam "go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
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

func TestNavSetup(t *testing.T) {
	ns, teardown := setupNavigationServiceFromConfig(t, "../data/nav_cfg.json")
	defer teardown()
	ctx := context.Background()

	navMode, err := ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, 0)

	err = ns.SetMode(ctx, 1, nil)
	test.That(t, err, test.ShouldBeNil)
	navMode, err = ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, 1)

	loc, err := ns.Location(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc, test.ShouldResemble, geo.NewPoint(40.7, -73.98))

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
}

func TestStartWaypoint(t *testing.T) {
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

	kinematicBase, err := fakeBase.(base.KinematicWrappable).WrapWithKinematics(ctx, fakeslam.NewSLAM(slam.Named("foo"), logger))
	test.That(t, err, test.ShouldBeNil)

	injectMovementSensor := inject.NewMovementSensor("test_movement")
	injectMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		inputs, err := kinematicBase.CurrentInputs(ctx)
		return geo.NewPoint(inputs[0].Value, inputs[1].Value), 0, err
	}

	injectMS.MoveOnGlobeFunc = func(
		ctx context.Context,
		componentName resource.Name,
		destination *geo.Point,
		heading float64,
		movementSensorName resource.Name,
		obstacles []*spatialmath.GeoObstacle,
		linearVelocity float64,
		angularVelocity float64,
		extra map[string]interface{},
	) (bool, error) {
		err := kinematicBase.GoToInputs(ctx, referenceframe.FloatsToInputs([]float64{destination.Lat(), destination.Lng(), 0}))
		fmt.Println(kinematicBase.CurrentInputs(ctx))
		return true, err
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

	pt := geo.NewPoint(1, 0)
	err = ns.AddWaypoint(ctx, pt, nil)
	test.That(t, err, test.ShouldBeNil)

	pt = geo.NewPoint(3, 1)
	err = ns.AddWaypoint(ctx, pt, nil)
	test.That(t, err, test.ShouldBeNil)

	err = ns.SetMode(ctx, 1, nil)

	test.That(t, err, test.ShouldBeNil)

	// TODO: find better way to await return from SetMode before running next test case
	ns.(*builtIn).activeBackgroundWorkers.Wait()

	inputs, err := kinematicBase.CurrentInputs(ctx)
	actualpt := geo.NewPoint(inputs[0].Value, inputs[1].Value)
	test.That(t, actualpt, test.ShouldResemble, pt)
}

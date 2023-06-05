// Package builtin contains the default navigation service, along with a gRPC server and client
package builtin

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	_ "go.viam.com/rdk/components/base/fake"
	_ "go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/config"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
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
	setupNavigationServiceFromConfig(t, "../data/nav_cfg.json")
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

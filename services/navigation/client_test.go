package navigation_test

import (
	"context"
	"math"
	"net"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	servicepb "go.viam.com/rdk/proto/api/service/navigation/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	failingServer := grpc.NewServer()

	workingNavigationService := &inject.NavigationService{}
	failingNavigationService := &inject.NavigationService{}

	modeTested := false
	workingNavigationService.GetModeFunc = func(ctx context.Context) (navigation.Mode, error) {
		if !modeTested {
			modeTested = true
			return navigation.ModeManual, nil
		}
		return navigation.ModeWaypoint, nil
	}
	var receivedMode navigation.Mode
	workingNavigationService.SetModeFunc = func(ctx context.Context, mode navigation.Mode) error {
		receivedMode = mode
		return nil
	}
	expectedLoc := geo.NewPoint(80, 1)
	workingNavigationService.GetLocationFunc = func(ctx context.Context) (*geo.Point, error) {
		return expectedLoc, nil
	}
	waypoints := []navigation.Waypoint{
		{
			ID:    primitive.NewObjectID(),
			Order: 0,
			Lat:   40,
			Long:  20,
		},
	}
	workingNavigationService.GetWaypointsFunc = func(ctx context.Context) ([]navigation.Waypoint, error) {
		return waypoints, nil
	}
	var receivedPoint *geo.Point
	workingNavigationService.AddWaypointFunc = func(ctx context.Context, point *geo.Point) error {
		receivedPoint = point
		return nil
	}
	var receivedID primitive.ObjectID
	workingNavigationService.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID) error {
		receivedID = id
		return nil
	}

	failingNavigationService.GetModeFunc = func(ctx context.Context) (navigation.Mode, error) {
		return navigation.ModeManual, errors.New("failure to retrieve mode")
	}
	var receivedFailingMode navigation.Mode
	failingNavigationService.SetModeFunc = func(ctx context.Context, mode navigation.Mode) error {
		receivedFailingMode = mode
		return errors.New("failure to set mode")
	}
	failingNavigationService.GetLocationFunc = func(ctx context.Context) (*geo.Point, error) {
		return nil, errors.New("failure to retrieve location")
	}
	failingNavigationService.GetWaypointsFunc = func(ctx context.Context) ([]navigation.Waypoint, error) {
		return nil, errors.New("failure to retrieve waypoints")
	}
	var receivedFailingPoint *geo.Point
	failingNavigationService.AddWaypointFunc = func(ctx context.Context, point *geo.Point) error {
		receivedFailingPoint = point
		return errors.New("failure to add waypoint")
	}
	var receivedFailingID primitive.ObjectID
	failingNavigationService.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID) error {
		receivedFailingID = id
		return errors.New("failure to remove waypoint")
	}

	workingSvc, err := subtype.New(map[resource.Name]interface{}{
		navigation.Named(testSvcName1): workingNavigationService,
	})
	test.That(t, err, test.ShouldBeNil)
	failingSvc, err := subtype.New(map[resource.Name]interface{}{
		navigation.Named(testSvcName1): failingNavigationService,
	})
	test.That(t, err, test.ShouldBeNil)

	resourceSubtype := registry.ResourceSubtypeLookup(navigation.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), workingServer, workingSvc)
	servicepb.RegisterNavigationServiceServer(failingServer, navigation.NewServer(failingSvc))

	go workingServer.Serve(listener1)
	defer workingServer.Stop()

	t.Run("context canceled", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingNavClient := navigation.NewClientFromConn(context.Background(), conn, testSvcName1, logger)

	t.Run("client tests for working navigation service", func(t *testing.T) {
		// test mode
		mode, err := workingNavClient.GetMode(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, navigation.ModeManual)
		mode, err = workingNavClient.GetMode(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, navigation.ModeWaypoint)

		// test set mode
		err = workingNavClient.SetMode(context.Background(), navigation.ModeManual)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedMode, test.ShouldEqual, navigation.ModeManual)

		// test add waypoint
		point := geo.NewPoint(90, 1)
		err = workingNavClient.AddWaypoint(context.Background(), point)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedPoint, test.ShouldResemble, point)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working navigation service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient := navigation.NewClientFromConn(context.Background(), conn, testSvcName1, logger)

		// test location
		loc, err := workingDialedClient.GetLocation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, loc, test.ShouldResemble, expectedLoc)

		// test remove waypoint
		wptID := primitive.NewObjectID()
		err = workingDialedClient.RemoveWaypoint(context.Background(), wptID)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, wptID, test.ShouldEqual, receivedID)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client test 2 for working navigation service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		workingDialedClient, ok := dialedClient.(navigation.Service)
		test.That(t, ok, test.ShouldBeTrue)

		// test waypoints
		receivedWpts, err := workingDialedClient.GetWaypoints(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedWpts, test.ShouldResemble, waypoints)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	go failingServer.Serve(listener2)
	defer failingServer.Stop()

	t.Run("client tests for failing navigation service", func(t *testing.T) {
		conn, err = viamgrpc.Dial(context.Background(), listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingNavClient := navigation.NewClientFromConn(context.Background(), conn, testSvcName1, logger)

		// test mode
		_, err := failingNavClient.GetMode(context.Background())
		test.That(t, err, test.ShouldNotBeNil)

		// test set mode
		err = failingNavClient.SetMode(context.Background(), navigation.ModeWaypoint)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, receivedFailingMode, test.ShouldEqual, navigation.ModeWaypoint)
		err = failingNavClient.SetMode(context.Background(), navigation.Mode(math.MaxUint8))
		test.That(t, err, test.ShouldNotBeNil)

		// test add waypoint
		point := geo.NewPoint(90, 1)
		err = failingNavClient.AddWaypoint(context.Background(), point)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, receivedFailingPoint, test.ShouldResemble, point)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client test for failing navigation service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		failingDialedClient, ok := dialedClient.(navigation.Service)
		test.That(t, ok, test.ShouldBeTrue)

		// test waypoints
		_, err = failingDialedClient.GetWaypoints(context.Background())
		test.That(t, err, test.ShouldNotBeNil)

		// test location
		loc, err := failingDialedClient.GetLocation(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, loc, test.ShouldBeNil)

		// test remove waypoint
		wptID := primitive.NewObjectID()
		err = failingDialedClient.RemoveWaypoint(context.Background(), wptID)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, wptID, test.ShouldEqual, receivedFailingID)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

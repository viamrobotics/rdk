package navigation_test

import (
	"context"
	"math"
	"net"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	servicepb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testSvcName1 = navigation.Named("nav1")
	testSvcName2 = navigation.Named("nav2")
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	listener2, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	failingServer := grpc.NewServer()

	var extraOptions map[string]interface{}
	workingNavigationService := &inject.NavigationService{}
	failingNavigationService := &inject.NavigationService{}

	modeTested := false
	workingNavigationService.ModeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
		extraOptions = extra
		if !modeTested {
			modeTested = true
			return navigation.ModeManual, nil
		}
		return navigation.ModeWaypoint, nil
	}
	var receivedMode navigation.Mode
	workingNavigationService.SetModeFunc = func(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
		extraOptions = extra
		receivedMode = mode
		return nil
	}
	expectedLoc := geo.NewPoint(80, 1)
	expectedCompassHeading := 90.
	expectedGeoPose := spatialmath.NewGeoPose(expectedLoc, expectedCompassHeading)
	workingNavigationService.LocationFunc = func(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
		extraOptions = extra

		return expectedGeoPose, nil
	}
	waypoints := []navigation.Waypoint{
		{
			ID:    primitive.NewObjectID(),
			Order: 0,
			Lat:   40,
			Long:  20,
		},
	}
	workingNavigationService.WaypointsFunc = func(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
		extraOptions = extra
		return waypoints, nil
	}
	var receivedPoint *geo.Point
	workingNavigationService.AddWaypointFunc = func(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
		extraOptions = extra
		receivedPoint = point
		return nil
	}
	var receivedID primitive.ObjectID
	workingNavigationService.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
		extraOptions = extra
		receivedID = id
		return nil
	}

	var receivedPaths []*navigation.Path
	workingNavigationService.PathsFunc = func(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error) {
		path, err := navigation.NewPath(primitive.NewObjectID(), []*geo.Point{geo.NewPoint(0, 0)})
		test.That(t, err, test.ShouldBeNil)
		receivedPaths = []*navigation.Path{path}
		return receivedPaths, nil
	}

	receivedProp := navigation.Properties{MapType: navigation.NoMap}
	workingNavigationService.PropertiesFunc = func(ctx context.Context) (navigation.Properties, error) {
		return receivedProp, nil
	}

	failingNavigationService.ModeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
		return navigation.ModeManual, errors.New("failure to retrieve mode")
	}
	var receivedFailingMode navigation.Mode
	failingNavigationService.SetModeFunc = func(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
		receivedFailingMode = mode
		return errors.New("failure to set mode")
	}
	failingNavigationService.LocationFunc = func(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
		return nil, errors.New("failure to retrieve location")
	}
	failingNavigationService.WaypointsFunc = func(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
		return nil, errors.New("failure to retrieve waypoints")
	}
	var receivedFailingPoint *geo.Point
	failingNavigationService.AddWaypointFunc = func(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
		receivedFailingPoint = point
		return errors.New("failure to add waypoint")
	}
	var receivedFailingID primitive.ObjectID
	failingNavigationService.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
		receivedFailingID = id
		return errors.New("failure to remove waypoint")
	}
	failingNavigationService.PathsFunc = func(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error) {
		return nil, errors.New("unimplemented")
	}

	failingNavigationService.PropertiesFunc = func(ctx context.Context) (navigation.Properties, error) {
		return navigation.Properties{}, errors.New("unimplemented")
	}

	workingSvc, err := resource.NewAPIResourceCollection(navigation.API, map[resource.Name]navigation.Service{
		testSvcName1: workingNavigationService,
	})
	test.That(t, err, test.ShouldBeNil)
	failingSvc, err := resource.NewAPIResourceCollection(navigation.API, map[resource.Name]navigation.Service{
		testSvcName1: failingNavigationService,
	})
	test.That(t, err, test.ShouldBeNil)

	resourceAPI, ok, err := resource.LookupAPIRegistration[navigation.Service](navigation.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	resourceAPI.RegisterRPCService(context.Background(), workingServer, workingSvc)
	failingServer.RegisterService(&servicepb.NavigationService_ServiceDesc, navigation.NewRPCServiceServer(failingSvc))

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
	workingNavClient, err := navigation.NewClientFromConn(context.Background(), conn, "", testSvcName1, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for working navigation service", func(t *testing.T) {
		// test mode
		extra := map[string]interface{}{"foo": "Mode"}
		mode, err := workingNavClient.Mode(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, navigation.ModeManual)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		mode, err = workingNavClient.Mode(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, navigation.ModeWaypoint)
		test.That(t, extraOptions, test.ShouldResemble, map[string]interface{}{})

		// test set mode
		extra = map[string]interface{}{"foo": "SetMode"}
		err = workingNavClient.SetMode(context.Background(), navigation.ModeManual, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedMode, test.ShouldEqual, navigation.ModeManual)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		err = workingNavClient.SetMode(context.Background(), navigation.ModeWaypoint, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedMode, test.ShouldEqual, navigation.ModeWaypoint)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		err = workingNavClient.SetMode(context.Background(), navigation.ModeExplore, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedMode, test.ShouldEqual, navigation.ModeExplore)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		err = workingNavClient.SetMode(context.Background(), 99, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "MODE_UNSPECIFIED")

		// test add waypoint
		point := geo.NewPoint(90, 1)
		extra = map[string]interface{}{"foo": "AddWaypoint"}
		err = workingNavClient.AddWaypoint(context.Background(), point, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedPoint, test.ShouldResemble, point)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		// test Paths
		ctx := context.Background()
		paths, err := workingNavClient.Paths(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, paths, test.ShouldResemble, receivedPaths)

		// test Properties
		prop, err := workingNavClient.Properties(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, prop.MapType, test.ShouldEqual, receivedProp.MapType)

		// test do command
		workingNavigationService.DoCommandFunc = testutils.EchoFunc
		resp, err := workingNavClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working navigation service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDialedClient, err := navigation.NewClientFromConn(context.Background(), conn, "", testSvcName1, logger)
		test.That(t, err, test.ShouldBeNil)

		// test location
		extra := map[string]interface{}{"foo": "Location"}
		geoPose, err := workingDialedClient.Location(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, geoPose, test.ShouldResemble, expectedGeoPose)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		// test remove waypoint
		wptID := primitive.NewObjectID()
		extra = map[string]interface{}{"foo": "RemoveWaypoint"}
		err = workingDialedClient.RemoveWaypoint(context.Background(), wptID, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, wptID, test.ShouldEqual, receivedID)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client test 2 for working navigation service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient, err := resourceAPI.RPCClient(context.Background(), conn, "", testSvcName1, logger)
		test.That(t, err, test.ShouldBeNil)

		// test waypoints
		extra := map[string]interface{}{"foo": "Waypoints"}
		receivedWpts, err := dialedClient.Waypoints(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, receivedWpts, test.ShouldResemble, waypoints)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	go failingServer.Serve(listener2)
	defer failingServer.Stop()

	t.Run("client tests for failing navigation service", func(t *testing.T) {
		conn, err = viamgrpc.Dial(context.Background(), listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingNavClient, err := navigation.NewClientFromConn(context.Background(), conn, "", testSvcName1, logger)
		test.That(t, err, test.ShouldBeNil)

		// test mode
		_, err = failingNavClient.Mode(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)

		// test set mode
		err = failingNavClient.SetMode(context.Background(), navigation.ModeWaypoint, map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, receivedFailingMode, test.ShouldEqual, navigation.ModeWaypoint)
		err = failingNavClient.SetMode(context.Background(), navigation.Mode(math.MaxUint8), map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)

		// test add waypoint
		point := geo.NewPoint(90, 1)
		err = failingNavClient.AddWaypoint(context.Background(), point, map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, receivedFailingPoint, test.ShouldResemble, point)

		// test Paths
		paths, err := failingNavClient.Paths(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, paths, test.ShouldBeNil)

		// test Properties
		prop, err := failingNavClient.Properties(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, prop, test.ShouldResemble, navigation.Properties{})

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client test for failing navigation service", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener2.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		dialedClient, err := resourceAPI.RPCClient(context.Background(), conn, "", testSvcName1, logger)
		test.That(t, err, test.ShouldBeNil)

		// test waypoints
		_, err = dialedClient.Waypoints(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)

		// test location
		geoPose, err := dialedClient.Location(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, geoPose, test.ShouldBeNil)

		// test remove waypoint
		wptID := primitive.NewObjectID()
		err = dialedClient.RemoveWaypoint(context.Background(), wptID, map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, wptID, test.ShouldEqual, receivedFailingID)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

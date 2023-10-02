package navigation_test

import (
	"context"
	"errors"
	"math"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func createWaypoints() ([]navigation.Waypoint, []*pb.Waypoint) {
	waypoints := []navigation.Waypoint{
		{
			ID:      primitive.NewObjectID(),
			Visited: true,
			Order:   0,
			Lat:     40,
			Long:    20,
		},
		{
			ID:      primitive.NewObjectID(),
			Visited: true,
			Order:   1,
			Lat:     50,
			Long:    30,
		},
		{
			ID:      primitive.NewObjectID(),
			Visited: false,
			Order:   2,
			Lat:     60,
			Long:    40,
		},
	}
	protoWaypoints := make([]*pb.Waypoint, 0, len(waypoints))
	for _, wp := range waypoints {
		protoWaypoints = append(protoWaypoints, &pb.Waypoint{
			Id: wp.ID.Hex(),
			Location: &commonpb.GeoPoint{
				Latitude:  wp.Lat,
				Longitude: wp.Long,
			},
		})
	}
	return waypoints, protoWaypoints
}

func TestServer(t *testing.T) {
	injectSvc := &inject.NavigationService{}
	resourceMap := map[resource.Name]navigation.Service{
		testSvcName1: injectSvc,
		testSvcName2: injectSvc,
	}
	injectAPISvc, err := resource.NewAPIResourceCollection(navigation.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	navServer := navigation.NewRPCServiceServer(injectAPISvc).(pb.NavigationServiceServer)

	var extraOptions map[string]interface{}
	t.Run("working mode function", func(t *testing.T) {
		// manual mode
		injectSvc.ModeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
			extraOptions = extra
			return navigation.ModeManual, nil
		}

		extra := map[string]interface{}{"foo": "Mode"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		req := &pb.GetModeRequest{Name: testSvcName1.ShortName(), Extra: ext}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_MANUAL)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		// waypoint mode
		injectSvc.ModeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
			return navigation.ModeWaypoint, nil
		}
		req = &pb.GetModeRequest{Name: testSvcName1.ShortName()}
		resp, err = navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_WAYPOINT)

		// return unspecified mode when returned mode unrecognized
		injectSvc.ModeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
			return navigation.Mode(math.MaxUint8), nil
		}
		req = &pb.GetModeRequest{Name: testSvcName1.ShortName()}
		resp, err = navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_UNSPECIFIED)
	})

	t.Run("failing mode function", func(t *testing.T) {
		injectSvc.ModeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
			return 0, errors.New("mode failed")
		}
		req := &pb.GetModeRequest{Name: testSvcName1.ShortName()}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working set mode function", func(t *testing.T) {
		var currentMode navigation.Mode
		injectSvc.SetModeFunc = func(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
			extraOptions = extra
			currentMode = mode
			return nil
		}
		extra := map[string]interface{}{"foo": "SetMode"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		// set manual mode
		req := &pb.SetModeRequest{
			Name:  testSvcName1.ShortName(),
			Mode:  pb.Mode_MODE_MANUAL,
			Extra: ext,
		}
		resp, err := navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, currentMode, test.ShouldEqual, navigation.ModeManual)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		// set waypoint mode
		req = &pb.SetModeRequest{
			Name: testSvcName1.ShortName(),
			Mode: pb.Mode_MODE_WAYPOINT,
		}
		resp, err = navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, currentMode, test.ShouldEqual, navigation.ModeWaypoint)

		// set explore mode
		req = &pb.SetModeRequest{
			Name: testSvcName1.ShortName(),
			Mode: pb.Mode_MODE_EXPLORE,
		}
		resp, err = navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, currentMode, test.ShouldEqual, navigation.ModeExplore)
	})

	t.Run("failing set mode function", func(t *testing.T) {
		// internal set mode failure
		injectSvc.SetModeFunc = func(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
			return errors.New("failed to set mode")
		}
		req := &pb.SetModeRequest{
			Name: testSvcName1.ShortName(),
			Mode: pb.Mode_MODE_MANUAL,
		}
		resp, err := navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)

		// unspecified mode passed
		injectSvc.SetModeFunc = func(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
			return nil
		}
		req = &pb.SetModeRequest{
			Name: testSvcName1.ShortName(),
			Mode: pb.Mode_MODE_UNSPECIFIED,
		}
		resp, err = navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working location function", func(t *testing.T) {
		loc := geo.NewPoint(90, 1)
		expectedCompassHeading := 90.
		expectedGeoPose := spatialmath.NewGeoPose(loc, expectedCompassHeading)
		injectSvc.LocationFunc = func(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
			extraOptions = extra
			return expectedGeoPose, nil
		}
		extra := map[string]interface{}{"foo": "Location"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		req := &pb.GetLocationRequest{Name: testSvcName1.ShortName(), Extra: ext}
		resp, err := navServer.GetLocation(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		protoLoc := resp.GetLocation()
		test.That(t, protoLoc.GetLatitude(), test.ShouldEqual, loc.Lat())
		test.That(t, protoLoc.GetLongitude(), test.ShouldEqual, loc.Lng())
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, resp.GetCompassHeading(), test.ShouldEqual, 90.)
	})

	t.Run("failing location function", func(t *testing.T) {
		injectSvc.LocationFunc = func(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
			return nil, errors.New("location retrieval failed")
		}
		req := &pb.GetLocationRequest{Name: testSvcName1.ShortName()}
		resp, err := navServer.GetLocation(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working waypoints function", func(t *testing.T) {
		waypoints, expectedResp := createWaypoints()
		injectSvc.WaypointsFunc = func(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
			extraOptions = extra
			return waypoints, nil
		}
		extra := map[string]interface{}{"foo": "Waypoints"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		req := &pb.GetWaypointsRequest{Name: testSvcName1.ShortName(), Extra: ext}
		resp, err := navServer.GetWaypoints(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.GetWaypoints(), test.ShouldResemble, expectedResp)
		test.That(t, extraOptions, test.ShouldResemble, extra)
	})

	t.Run("failing waypoints function", func(t *testing.T) {
		injectSvc.WaypointsFunc = func(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
			return nil, errors.New("waypoints retrieval failed")
		}
		req := &pb.GetWaypointsRequest{Name: testSvcName1.ShortName()}
		resp, err := navServer.GetWaypoints(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working add waypoint", func(t *testing.T) {
		var receivedPoint geo.Point
		injectSvc.AddWaypointFunc = func(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
			extraOptions = extra
			receivedPoint = *point
			return nil
		}
		extra := map[string]interface{}{"foo": "AddWaypoint"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		req := &pb.AddWaypointRequest{
			Name: testSvcName1.ShortName(),
			Location: &commonpb.GeoPoint{
				Latitude:  90,
				Longitude: 0,
			},
			Extra: ext,
		}
		expectedLatitude := req.GetLocation().GetLatitude()
		expectedLongitude := req.GetLocation().GetLongitude()
		resp, err := navServer.AddWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, receivedPoint.Lat(), test.ShouldEqual, expectedLatitude)
		test.That(t, receivedPoint.Lng(), test.ShouldEqual, expectedLongitude)
		test.That(t, extraOptions, test.ShouldResemble, extra)
	})

	t.Run("failing add waypoint", func(t *testing.T) {
		addWaypointCalled := false
		injectSvc.AddWaypointFunc = func(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
			addWaypointCalled = true
			return errors.New("failed to add waypoint")
		}
		req := &pb.AddWaypointRequest{
			Name: testSvcName1.ShortName(),
			Location: &commonpb.GeoPoint{
				Latitude:  90,
				Longitude: 0,
			},
		}
		resp, err := navServer.AddWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, addWaypointCalled, test.ShouldBeTrue)
	})

	t.Run("working remove waypoint", func(t *testing.T) {
		var receivedID primitive.ObjectID
		injectSvc.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
			extraOptions = extra
			receivedID = id
			return nil
		}
		extra := map[string]interface{}{"foo": "Sync"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		objectID := primitive.NewObjectID()
		req := &pb.RemoveWaypointRequest{
			Name:  testSvcName1.ShortName(),
			Id:    objectID.Hex(),
			Extra: ext,
		}
		resp, err := navServer.RemoveWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, receivedID, test.ShouldEqual, objectID)
		test.That(t, extraOptions, test.ShouldResemble, extra)
	})

	t.Run("failing remove waypoint", func(t *testing.T) {
		// fail on bad hex
		injectSvc.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
			return nil
		}
		req := &pb.RemoveWaypointRequest{
			Name: testSvcName1.ShortName(),
			Id:   "not a hex",
		}
		resp, err := navServer.RemoveWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)

		// fail on failing function
		injectSvc.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
			return errors.New("failed to remove waypoint")
		}
		req = &pb.RemoveWaypointRequest{
			Name: testSvcName1.ShortName(),
			Id:   primitive.NewObjectID().Hex(),
		}
		resp, err = navServer.RemoveWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	injectAPISvc, _ = resource.NewAPIResourceCollection(navigation.API, map[resource.Name]navigation.Service{})
	navServer = navigation.NewRPCServiceServer(injectAPISvc).(pb.NavigationServiceServer)
	t.Run("failing on nonexistent server", func(t *testing.T) {
		req := &pb.GetModeRequest{Name: testSvcName1.ShortName()}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(testSvcName1))
	})
	t.Run("multiple services valid", func(t *testing.T) {
		injectSvc = &inject.NavigationService{}
		resourceMap = map[resource.Name]navigation.Service{
			testSvcName1: injectSvc,
			testSvcName2: injectSvc,
		}
		injectAPISvc, err = resource.NewAPIResourceCollection(navigation.API, resourceMap)
		test.That(t, err, test.ShouldBeNil)
		navServer = navigation.NewRPCServiceServer(injectAPISvc).(pb.NavigationServiceServer)
		injectSvc.ModeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
			return navigation.ModeManual, nil
		}
		req := &pb.GetModeRequest{Name: testSvcName1.ShortName()}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_MANUAL)
		req = &pb.GetModeRequest{Name: testSvcName2.ShortName()}
		resp, err = navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_MANUAL)
	})
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]navigation.Service{
		testSvcName1: &inject.NavigationService{
			DoCommandFunc: testutils.EchoFunc,
		},
	}
	injectAPISvc, err := resource.NewAPIResourceCollection(navigation.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	server := navigation.NewRPCServiceServer(injectAPISvc).(pb.NavigationServiceServer)

	cmd, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testSvcName1.ShortName(),
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}

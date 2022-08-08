package navigation_test

import (
	"context"
	"errors"
	"math"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/navigation/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
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
	resourceMap := map[resource.Name]interface{}{
		navigation.Named(testSvcName1): injectSvc,
		navigation.Named(testSvcName2): injectSvc,
	}
	injectSubtypeSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	navServer := navigation.NewServer(injectSubtypeSvc)

	t.Run("working mode function", func(t *testing.T) {
		// manual mode
		injectSvc.GetModeFunc = func(ctx context.Context) (navigation.Mode, error) {
			return navigation.ModeManual, nil
		}
		req := &pb.GetModeRequest{Name: testSvcName1}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_MANUAL)

		// waypoint mode
		injectSvc.GetModeFunc = func(ctx context.Context) (navigation.Mode, error) {
			return navigation.ModeWaypoint, nil
		}
		req = &pb.GetModeRequest{Name: testSvcName1}
		resp, err = navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_WAYPOINT)

		// return unspecified mode when returned mode unrecognized
		injectSvc.GetModeFunc = func(ctx context.Context) (navigation.Mode, error) {
			return navigation.Mode(math.MaxUint8), nil
		}
		req = &pb.GetModeRequest{Name: testSvcName1}
		resp, err = navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Mode, test.ShouldEqual, pb.Mode_MODE_UNSPECIFIED)
	})

	t.Run("failing mode function", func(t *testing.T) {
		injectSvc.GetModeFunc = func(ctx context.Context) (navigation.Mode, error) {
			return 0, errors.New("mode failed")
		}
		req := &pb.GetModeRequest{Name: testSvcName1}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working set mode function", func(t *testing.T) {
		var currentMode navigation.Mode
		injectSvc.SetModeFunc = func(ctx context.Context, mode navigation.Mode) error {
			currentMode = mode
			return nil
		}

		// set manual mode
		req := &pb.SetModeRequest{
			Name: testSvcName1,
			Mode: pb.Mode_MODE_MANUAL,
		}
		resp, err := navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, currentMode, test.ShouldEqual, navigation.ModeManual)

		// set waypoint mode
		req = &pb.SetModeRequest{
			Name: testSvcName1,
			Mode: pb.Mode_MODE_WAYPOINT,
		}
		resp, err = navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, currentMode, test.ShouldEqual, navigation.ModeWaypoint)
	})

	t.Run("failing set mode function", func(t *testing.T) {
		// internal set mode failure
		injectSvc.SetModeFunc = func(ctx context.Context, mode navigation.Mode) error {
			return errors.New("failed to set mode")
		}
		req := &pb.SetModeRequest{
			Name: testSvcName1,
			Mode: pb.Mode_MODE_MANUAL,
		}
		resp, err := navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)

		// unspecified mode passed
		injectSvc.SetModeFunc = func(ctx context.Context, mode navigation.Mode) error {
			return nil
		}
		req = &pb.SetModeRequest{
			Name: testSvcName1,
			Mode: pb.Mode_MODE_UNSPECIFIED,
		}
		resp, err = navServer.SetMode(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working location function", func(t *testing.T) {
		loc := geo.NewPoint(90, 1)
		injectSvc.GetLocationFunc = func(ctx context.Context) (*geo.Point, error) {
			return loc, nil
		}
		req := &pb.GetLocationRequest{Name: testSvcName1}
		resp, err := navServer.GetLocation(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		protoLoc := resp.GetLocation()
		test.That(t, protoLoc.GetLatitude(), test.ShouldEqual, loc.Lat())
		test.That(t, protoLoc.GetLongitude(), test.ShouldEqual, loc.Lng())
	})

	t.Run("failing location function", func(t *testing.T) {
		injectSvc.GetLocationFunc = func(ctx context.Context) (*geo.Point, error) {
			return nil, errors.New("location retrieval failed")
		}
		req := &pb.GetLocationRequest{Name: testSvcName1}
		resp, err := navServer.GetLocation(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working waypoints function", func(t *testing.T) {
		waypoints, expectedResp := createWaypoints()
		injectSvc.GetWaypointsFunc = func(ctx context.Context) ([]navigation.Waypoint, error) {
			return waypoints, nil
		}
		req := &pb.GetWaypointsRequest{Name: testSvcName1}
		resp, err := navServer.GetWaypoints(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.GetWaypoints(), test.ShouldResemble, expectedResp)
	})

	t.Run("failing waypoints function", func(t *testing.T) {
		injectSvc.GetWaypointsFunc = func(ctx context.Context) ([]navigation.Waypoint, error) {
			return nil, errors.New("waypoints retrieval failed")
		}
		req := &pb.GetWaypointsRequest{Name: testSvcName1}
		resp, err := navServer.GetWaypoints(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	t.Run("working add waypoint", func(t *testing.T) {
		var receivedPoint geo.Point
		injectSvc.AddWaypointFunc = func(ctx context.Context, point *geo.Point) error {
			receivedPoint = *point
			return nil
		}
		req := &pb.AddWaypointRequest{
			Name: testSvcName1,
			Location: &commonpb.GeoPoint{
				Latitude:  90,
				Longitude: 0,
			},
		}
		expectedLatitude := req.GetLocation().GetLatitude()
		expectedLongitude := req.GetLocation().GetLongitude()
		resp, err := navServer.AddWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, receivedPoint.Lat(), test.ShouldEqual, expectedLatitude)
		test.That(t, receivedPoint.Lng(), test.ShouldEqual, expectedLongitude)
	})

	t.Run("failing add waypoint", func(t *testing.T) {
		addWaypointCalled := false
		injectSvc.AddWaypointFunc = func(ctx context.Context, point *geo.Point) error {
			addWaypointCalled = true
			return errors.New("failed to add waypoint")
		}
		req := &pb.AddWaypointRequest{
			Name: testSvcName1,
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
		injectSvc.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID) error {
			receivedID = id
			return nil
		}
		objectID := primitive.NewObjectID()
		req := &pb.RemoveWaypointRequest{
			Name: testSvcName1,
			Id:   objectID.Hex(),
		}
		resp, err := navServer.RemoveWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, receivedID, test.ShouldEqual, objectID)
	})

	t.Run("failing remove waypoint", func(t *testing.T) {
		// fail on bad hex
		injectSvc.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID) error {
			return nil
		}
		req := &pb.RemoveWaypointRequest{
			Name: testSvcName1,
			Id:   "not a hex",
		}
		resp, err := navServer.RemoveWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)

		// fail on failing function
		injectSvc.RemoveWaypointFunc = func(ctx context.Context, id primitive.ObjectID) error {
			return errors.New("failed to remove waypoint")
		}
		req = &pb.RemoveWaypointRequest{
			Name: testSvcName1,
			Id:   primitive.NewObjectID().Hex(),
		}
		resp, err = navServer.RemoveWaypoint(context.Background(), req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, resp, test.ShouldBeNil)
	})

	resourceMap = map[resource.Name]interface{}{
		navigation.Named(testSvcName1): "not a frame system",
	}
	injectSubtypeSvc, _ = subtype.New(resourceMap)
	navServer = navigation.NewServer(injectSubtypeSvc)

	t.Run("failing on improper service interface", func(t *testing.T) {
		improperImplErr := utils.NewUnimplementedInterfaceError("navigation.Service", "string")

		getModeReq := &pb.GetModeRequest{Name: testSvcName1}
		getModeResp, err := navServer.GetMode(context.Background(), getModeReq)
		test.That(t, getModeResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)

		setModeReq := &pb.SetModeRequest{
			Name: testSvcName1,
			Mode: pb.Mode_MODE_MANUAL,
		}
		setModeResp, err := navServer.SetMode(context.Background(), setModeReq)
		test.That(t, err, test.ShouldBeError, improperImplErr)
		test.That(t, setModeResp, test.ShouldBeNil)

		getLocReq := &pb.GetLocationRequest{Name: testSvcName1}
		getLocResp, err := navServer.GetLocation(context.Background(), getLocReq)
		test.That(t, err, test.ShouldBeError, improperImplErr)
		test.That(t, getLocResp, test.ShouldBeNil)

		waypointReq := &pb.GetWaypointsRequest{Name: testSvcName1}
		waypointResp, err := navServer.GetWaypoints(context.Background(), waypointReq)
		test.That(t, err, test.ShouldBeError, improperImplErr)
		test.That(t, waypointResp, test.ShouldBeNil)

		addWptReq := &pb.AddWaypointRequest{Name: testSvcName1}
		addWptResp, err := navServer.AddWaypoint(context.Background(), addWptReq)
		test.That(t, err, test.ShouldBeError, improperImplErr)
		test.That(t, addWptResp, test.ShouldBeNil)

		remWptReq := &pb.RemoveWaypointRequest{Name: testSvcName1}
		remWptResp, err := navServer.RemoveWaypoint(context.Background(), remWptReq)
		test.That(t, remWptResp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, improperImplErr)
	})

	injectSubtypeSvc, _ = subtype.New(map[resource.Name]interface{}{})
	navServer = navigation.NewServer(injectSubtypeSvc)
	t.Run("failing on nonexistent server", func(t *testing.T) {
		req := &pb.GetModeRequest{Name: testSvcName1}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(navigation.Named(testSvcName1)))
	})
	t.Run("multiple services valid", func(t *testing.T) {
		req := &pb.GetModeRequest{Name: testSvcName1}
		resp, err := navServer.GetMode(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(navigation.Named(testSvcName1)))
		req = &pb.GetModeRequest{Name: testSvcName2}
		resp, err = navServer.GetMode(context.Background(), req)
		test.That(t, resp, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(navigation.Named(testSvcName2)))
	})
}

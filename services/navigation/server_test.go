package navigation_test

import (
	"context"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestServerGetProperties(t *testing.T) {
	svc := &inject.NavigationService{}
	svc.GetPropertiesFunc = func(ctx context.Context) (navigation.Properties, error) {
		return navigation.Properties{
			MapType:           navigation.GPSMap,
			PlanningSupported: true,
			RoutesSupported:   true,
		}, nil
	}
	navName := navigation.Named("nav1")
	coll, err := resource.NewAPIResourceCollection(navigation.API, map[resource.Name]navigation.Service{
		navName: svc,
	})
	test.That(t, err, test.ShouldBeNil)
	server := navigation.NewRPCServiceServer(coll, logging.NewTestLogger(t)).(pb.NavigationServiceServer)

	resp, err := server.GetProperties(context.Background(), &pb.GetPropertiesRequest{Name: navName.Name})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetProperties().GetMapType(), test.ShouldEqual, pb.MapType_MAP_TYPE_GPS)
	test.That(t, resp.GetProperties().GetPlanningSupported(), test.ShouldBeTrue)
}

func TestServerAddLocation(t *testing.T) {
	svc := &inject.NavigationService{}
	svc.AddLocationFunc = func(ctx context.Context, req navigation.AddLocationReq) (navigation.Location, error) {
		test.That(t, req.Position, test.ShouldNotBeNil)
		test.That(t, req.Position.GeoPose, test.ShouldNotBeNil)
		return navigation.Location{Name: req.Name, Position: *req.Position}, nil
	}
	navName := navigation.Named("nav1")
	coll, err := resource.NewAPIResourceCollection(navigation.API, map[resource.Name]navigation.Service{
		navName: svc,
	})
	test.That(t, err, test.ShouldBeNil)
	server := navigation.NewRPCServiceServer(coll, logging.NewTestLogger(t)).(pb.NavigationServiceServer)

	heading := 10.
	resp, err := server.AddLocation(context.Background(), &pb.AddLocationRequest{
		Name:         navName.Name,
		LocationName: "gate",
		Position: &pb.Position{Value: &pb.Position_GeoPose{GeoPose: &commonpb.GeoPose{
			Location:    &commonpb.GeoPoint{Latitude: 40.7, Longitude: -74},
			Orientation: &commonpb.GeoPose_Heading{Heading: heading},
		}}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.GetLocation().GetName(), test.ShouldEqual, "gate")
	test.That(t, resp.GetLocation().GetPosition().GetGeoPose().GetHeading(), test.ShouldEqual, heading)
}

func TestServerAddLocationOrientationVector(t *testing.T) {
	svc := &inject.NavigationService{}
	svc.AddLocationFunc = func(ctx context.Context, req navigation.AddLocationReq) (navigation.Location, error) {
		test.That(t, req.Position.GeoPose.Heading, test.ShouldBeNil)
		test.That(t, req.Position.GeoPose.Orientation.OZ, test.ShouldEqual, 1.)
		test.That(t, req.Position.GeoPose.Orientation.Theta, test.ShouldEqual, 45.)
		return navigation.Location{Name: req.Name, Position: *req.Position}, nil
	}
	server, navName := newTestServer(t, svc)

	resp, err := server.AddLocation(context.Background(), &pb.AddLocationRequest{
		Name:         navName.Name,
		LocationName: "dock",
		Position: &pb.Position{Value: &pb.Position_GeoPose{GeoPose: &commonpb.GeoPose{
			Location: &commonpb.GeoPoint{Latitude: 40.7, Longitude: -74},
			Orientation: &commonpb.GeoPose_OrientationVector{OrientationVector: &commonpb.Orientation{
				OX: 0, OY: 0, OZ: 1, Theta: 45,
			}},
		}}},
	})
	test.That(t, err, test.ShouldBeNil)
	ov := resp.GetLocation().GetPosition().GetGeoPose().GetOrientationVector()
	test.That(t, ov.GetOZ(), test.ShouldEqual, 1.)
	test.That(t, ov.GetTheta(), test.ShouldEqual, 45.)
	test.That(t, resp.GetLocation().GetPosition().GetGeoPose().GetHeading(), test.ShouldEqual, 0.)
}

func TestServerUpdateRouteWaypoints(t *testing.T) {
	svc := &inject.NavigationService{}
	var gotNil bool
	svc.UpdateRouteFunc = func(ctx context.Context, req navigation.UpdateRouteReq) (navigation.Route, error) {
		gotNil = req.Waypoints == nil
		wps := req.Waypoints
		if wps == nil {
			wps = []navigation.Waypoint{{LocationName: "kept"}}
		}
		return navigation.Route{Name: req.Name, Waypoints: wps}, nil
	}
	server, navName := newTestServer(t, svc)

	resp, err := server.UpdateRoute(context.Background(), &pb.UpdateRouteRequest{
		Name:      navName.Name,
		RouteName: "loop",
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotNil, test.ShouldBeTrue)
	test.That(t, resp.GetRoute().GetWaypoints(), test.ShouldHaveLength, 1)

	resp, err = server.UpdateRoute(context.Background(), &pb.UpdateRouteRequest{
		Name:      navName.Name,
		RouteName: "loop",
		Waypoints: &pb.WaypointList{},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotNil, test.ShouldBeFalse)
	test.That(t, resp.GetRoute().GetWaypoints(), test.ShouldBeEmpty)
}

func TestServerStatusRPCs(t *testing.T) {
	svc := &inject.NavigationService{}
	svc.GetNavigationStatusFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Status, error) {
		return navigation.Status{
			Position:  navigation.Position{Pose: spatialmath.NewZeroPose()},
			Localized: true,
			Goal: &navigation.GoalStatus{
				State:  navigation.GoalStateActive,
				Mode:   navigation.NavModePlanned,
				Target: navigation.Position{Pose: spatialmath.NewZeroPose()},
			},
		}, nil
	}
	svc.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"localized": true}, nil
	}
	server, navName := newTestServer(t, svc)

	navResp, err := server.GetNavigationStatus(context.Background(), &pb.GetNavigationStatusRequest{Name: navName.Name})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navResp.GetStatus().GetLocalized(), test.ShouldBeTrue)
	test.That(t, navResp.GetStatus().GetGoal().GetState(), test.ShouldEqual, pb.GoalState_GOAL_STATE_ACTIVE)
	test.That(t, navResp.GetStatus().GetGoal().GetMode(), test.ShouldEqual, pb.NavMode_NAV_MODE_PLANNED)

	statusResp, err := server.GetStatus(context.Background(), &commonpb.GetStatusRequest{Name: navName.Name})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, statusResp.GetResult().AsMap()["localized"], test.ShouldEqual, true)
}

func newTestServer(t *testing.T, svc navigation.Service) (pb.NavigationServiceServer, resource.Name) {
	t.Helper()
	navName := navigation.Named("nav1")
	coll, err := resource.NewAPIResourceCollection(navigation.API, map[resource.Name]navigation.Service{
		navName: svc,
	})
	test.That(t, err, test.ShouldBeNil)
	server := navigation.NewRPCServiceServer(coll, logging.NewTestLogger(t)).(pb.NavigationServiceServer)
	return server, navName
}

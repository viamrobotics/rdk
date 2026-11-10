package navigation_test

import (
	"context"
	"net"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	servicepb "go.viam.com/api/service/navigation/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestClientAddAndGetLocation(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	heading := 90.
	expected := navigation.Location{
		Name: "dock",
		Position: navigation.Position{
			GeoPose: &navigation.GeoPose{
				Location: geo.NewPoint(40.7, -74.0),
				Heading:  &heading,
			},
		},
	}
	svc := &inject.NavigationService{}
	svc.AddLocationFunc = func(ctx context.Context, req navigation.AddLocationReq) (navigation.Location, error) {
		test.That(t, req.Name, test.ShouldEqual, "dock")
		return expected, nil
	}
	svc.GetLocationFunc = func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Location, error) {
		test.That(t, name, test.ShouldEqual, "dock")
		return expected, nil
	}
	svc.GetPropertiesFunc = func(ctx context.Context) (navigation.Properties, error) {
		return navigation.Properties{MapType: navigation.GPSMap, RoutesSupported: true}, nil
	}

	navName := navigation.Named("nav1")
	resourceMap := map[resource.Name]navigation.Service{navName: svc}
	navServer, err := resource.NewAPIResourceCollection(navigation.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[navigation.Service](navigation.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	resourceAPI.RegisterRPCService(context.Background(), workingServer, navServer, logger)
	go workingServer.Serve(listener)
	defer workingServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()
	client, err := navigation.NewClientFromConn(context.Background(), conn, "", navName, logger)
	test.That(t, err, test.ShouldBeNil)

	got, err := client.AddLocation(context.Background(), navigation.AddLocationReq{Name: "dock", Position: &expected.Position})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Name, test.ShouldEqual, "dock")

	got, err = client.GetLocation(context.Background(), "dock", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *got.Position.GeoPose.Heading, test.ShouldEqual, 90.)

	props, err := client.GetProperties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.MapType, test.ShouldEqual, navigation.GPSMap)
}

func TestClientOrientationVector(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	alt := 12.5
	svc := &inject.NavigationService{}
	svc.AddLocationFunc = func(ctx context.Context, req navigation.AddLocationReq) (navigation.Location, error) {
		test.That(t, req.Position.GeoPose.Heading, test.ShouldBeNil)
		test.That(t, req.Position.GeoPose.Orientation.Theta, test.ShouldEqual, 45.)
		return navigation.Location{Name: req.Name, Position: *req.Position}, nil
	}
	navName := navigation.Named("nav1")
	resourceMap := map[resource.Name]navigation.Service{navName: svc}
	navServer, err := resource.NewAPIResourceCollection(navigation.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[navigation.Service](navigation.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	resourceAPI.RegisterRPCService(context.Background(), workingServer, navServer, logger)
	go workingServer.Serve(listener)
	defer workingServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()
	client, err := navigation.NewClientFromConn(context.Background(), conn, "", navName, logger)
	test.That(t, err, test.ShouldBeNil)

	got, err := client.AddLocation(context.Background(), navigation.AddLocationReq{
		Name: "dock",
		Position: &navigation.Position{GeoPose: &navigation.GeoPose{
			Location:    geo.NewPoint(40.7, -74.0),
			Altitude:    &alt,
			Orientation: &spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1, Theta: 45},
		}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Position.GeoPose.Heading, test.ShouldBeNil)
	test.That(t, *got.Position.GeoPose.Altitude, test.ShouldEqual, alt)
	test.That(t, got.Position.GeoPose.Orientation.OZ, test.ShouldEqual, 1.)
	test.That(t, got.Position.GeoPose.Orientation.Theta, test.ShouldEqual, 45.)

	heading := 10.
	_, err = client.AddLocation(context.Background(), navigation.AddLocationReq{
		Name: "bad",
		Position: &navigation.Position{GeoPose: &navigation.GeoPose{
			Location:    geo.NewPoint(40.7, -74.0),
			Heading:     &heading,
			Orientation: &spatialmath.OrientationVectorDegrees{OZ: 1},
		}},
	})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestClientRPCs(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	workingServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	pos := gpsPosition(40.7, -74)
	status := navigation.Status{Position: pos, Localized: true}
	locName := "dock"
	speed := 1.2
	wait := false
	newName := "pier"

	svc := &inject.NavigationService{}
	svc.ListLocationsFunc = func(ctx context.Context, extra map[string]interface{}) ([]navigation.Location, error) {
		return []navigation.Location{{Name: locName, Position: pos}}, nil
	}
	svc.UpdateLocationFunc = func(ctx context.Context, req navigation.UpdateLocationReq) (navigation.Location, error) {
		test.That(t, req.Name, test.ShouldEqual, locName)
		test.That(t, *req.NewName, test.ShouldEqual, newName)
		return navigation.Location{Name: *req.NewName, Position: *req.Position}, nil
	}
	svc.DeleteLocationFunc = func(ctx context.Context, name string, extra map[string]interface{}) error {
		test.That(t, name, test.ShouldEqual, newName)
		return nil
	}
	svc.AddRouteFunc = func(ctx context.Context, req navigation.AddRouteReq) (navigation.Route, error) {
		test.That(t, req.Waypoints, test.ShouldHaveLength, 1)
		test.That(t, req.Waypoints[0].LocationName, test.ShouldEqual, locName)
		return navigation.Route{Name: req.Name, Waypoints: req.Waypoints}, nil
	}
	svc.GetRouteFunc = func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Route, error) {
		return navigation.Route{Name: name, Waypoints: []navigation.Waypoint{{LocationName: locName}}}, nil
	}
	svc.ListRoutesFunc = func(ctx context.Context, extra map[string]interface{}) ([]navigation.Route, error) {
		return []navigation.Route{{Name: "loop"}}, nil
	}
	svc.UpdateRouteFunc = func(ctx context.Context, req navigation.UpdateRouteReq) (navigation.Route, error) {
		wps := req.Waypoints
		if wps == nil {
			wps = []navigation.Waypoint{{LocationName: "kept"}}
		}
		return navigation.Route{Name: req.Name, Waypoints: wps}, nil
	}
	svc.DeleteRouteFunc = func(ctx context.Context, name string, extra map[string]interface{}) error {
		test.That(t, name, test.ShouldEqual, "loop")
		return nil
	}
	svc.AddZoneFunc = func(ctx context.Context, req navigation.AddZoneReq) (navigation.Zone, error) {
		test.That(t, req.Type, test.ShouldEqual, navigation.ZoneTypeKeepout)
		return navigation.Zone{Name: req.Name, Type: req.Type}, nil
	}
	svc.GetZoneFunc = func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Zone, error) {
		return navigation.Zone{Name: name, Type: navigation.ZoneTypeSpeedLimit, MaxSpeed: &speed}, nil
	}
	svc.ListZonesFunc = func(ctx context.Context, zoneType string, extra map[string]interface{}) ([]navigation.Zone, error) {
		test.That(t, zoneType, test.ShouldEqual, navigation.ZoneTypeKeepout)
		return []navigation.Zone{{Name: "yard", Type: zoneType}}, nil
	}
	svc.UpdateZoneFunc = func(ctx context.Context, req navigation.UpdateZoneReq) (navigation.Zone, error) {
		test.That(t, *req.MaxSpeed, test.ShouldEqual, speed)
		return navigation.Zone{Name: req.Name, Type: navigation.ZoneTypeSpeedLimit, MaxSpeed: req.MaxSpeed}, nil
	}
	svc.DeleteZoneFunc = func(ctx context.Context, name string, extra map[string]interface{}) error {
		test.That(t, name, test.ShouldEqual, "yard")
		return nil
	}
	svc.NavigateToLocationFunc = func(ctx context.Context, name string, extra map[string]interface{}) (navigation.Status, error) {
		test.That(t, name, test.ShouldEqual, locName)
		return status, nil
	}
	svc.NavigateToPointFunc = func(
		ctx context.Context, destination navigation.Position, extra map[string]interface{},
	) (navigation.Status, error) {
		test.That(t, destination.GeoPose.Location.Lat(), test.ShouldEqual, pos.GeoPose.Location.Lat())
		return status, nil
	}
	svc.NavigateRouteFunc = func(ctx context.Context, req navigation.NavigateRouteReq) (navigation.Status, error) {
		test.That(t, req.Name, test.ShouldEqual, "loop")
		test.That(t, req.Loop, test.ShouldBeTrue)
		test.That(t, req.StartIndex, test.ShouldEqual, 1)
		return status, nil
	}
	svc.PlanToPointFunc = func(ctx context.Context, req navigation.PlanToPointReq) (navigation.Plan, error) {
		test.That(t, req.Start, test.ShouldNotBeNil)
		return navigation.Plan{Path: []navigation.Position{*req.Start, req.Destination}, LengthM: 12}, nil
	}
	svc.PlanToLocationFunc = func(ctx context.Context, req navigation.PlanToLocationReq) (navigation.Plan, error) {
		test.That(t, req.Name, test.ShouldEqual, locName)
		return navigation.Plan{Path: []navigation.Position{pos}, Location: &locName, LengthM: 4}, nil
	}
	svc.ExecutePlanFunc = func(ctx context.Context, plan navigation.Plan, extra map[string]interface{}) (navigation.Status, error) {
		test.That(t, plan.LengthM, test.ShouldEqual, 4)
		return status, nil
	}
	svc.GoToLocationFunc = func(ctx context.Context, req navigation.GoToLocationReq) (navigation.Status, error) {
		test.That(t, req.Name, test.ShouldEqual, locName)
		test.That(t, *req.Wait, test.ShouldBeFalse)
		test.That(t, *req.LinearMPerSec, test.ShouldEqual, speed)
		return status, nil
	}
	svc.GoToPointFunc = func(ctx context.Context, req navigation.GoToPointReq) (navigation.Status, error) {
		test.That(t, req.Destination.GeoPose.Location.Lng(), test.ShouldEqual, pos.GeoPose.Location.Lng())
		return status, nil
	}
	svc.SuspendFunc = func(ctx context.Context, reason string, extra map[string]interface{}) (navigation.Status, error) {
		test.That(t, reason, test.ShouldEqual, "pause")
		return status, nil
	}
	svc.ResumeFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Status, error) {
		return status, nil
	}
	svc.CancelFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Status, error) {
		return status, nil
	}
	svc.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Position, error) {
		return pos, nil
	}
	svc.GetNavigationStatusFunc = func(ctx context.Context, extra map[string]interface{}) (navigation.Status, error) {
		return status, nil
	}
	svc.GetCostmapFunc = func(ctx context.Context, req navigation.CostmapReq) (navigation.Costmap, error) {
		test.That(t, req.Layer, test.ShouldEqual, "local")
		test.That(t, req.Stride, test.ShouldEqual, 2)
		return navigation.Costmap{Available: true, Layer: req.Layer, Width: 3, Height: 4, Data: []byte{1, 2}}, nil
	}
	svc.DoCommandFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return cmd, nil
	}
	svc.StatusFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"localized": true}, nil
	}

	navName := navigation.Named("nav1")
	resourceMap := map[resource.Name]navigation.Service{navName: svc}
	navServer, err := resource.NewAPIResourceCollection(navigation.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[navigation.Service](navigation.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	resourceAPI.RegisterRPCService(context.Background(), workingServer, navServer, logger)
	go workingServer.Serve(listener)
	defer workingServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()
	client, err := navigation.NewClientFromConn(context.Background(), conn, "", navName, logger)
	test.That(t, err, test.ShouldBeNil)
	ctx := context.Background()

	locs, err := client.ListLocations(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, locs, test.ShouldHaveLength, 1)

	updated, err := client.UpdateLocation(ctx, navigation.UpdateLocationReq{
		Name: locName, NewName: &newName, Position: &pos,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, updated.Name, test.ShouldEqual, newName)
	test.That(t, client.DeleteLocation(ctx, newName, nil), test.ShouldBeNil)

	route, err := client.AddRoute(ctx, navigation.AddRouteReq{
		Name: "loop", Waypoints: []navigation.Waypoint{{LocationName: locName}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, route.Waypoints[0].LocationName, test.ShouldEqual, locName)
	route, err = client.GetRoute(ctx, "loop", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, route.Name, test.ShouldEqual, "loop")
	routes, err := client.ListRoutes(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, routes, test.ShouldHaveLength, 1)

	route, err = client.UpdateRoute(ctx, navigation.UpdateRouteReq{Name: "loop"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, route.Waypoints, test.ShouldHaveLength, 1)
	route, err = client.UpdateRoute(ctx, navigation.UpdateRouteReq{Name: "loop", Waypoints: []navigation.Waypoint{}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, route.Waypoints, test.ShouldBeEmpty)
	test.That(t, client.DeleteRoute(ctx, "loop", nil), test.ShouldBeNil)

	zone, err := client.AddZone(ctx, navigation.AddZoneReq{Name: "yard", Type: navigation.ZoneTypeKeepout})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, zone.Type, test.ShouldEqual, navigation.ZoneTypeKeepout)
	zone, err = client.GetZone(ctx, "yard", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *zone.MaxSpeed, test.ShouldEqual, speed)
	zones, err := client.ListZones(ctx, navigation.ZoneTypeKeepout, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, zones, test.ShouldHaveLength, 1)
	zone, err = client.UpdateZone(ctx, navigation.UpdateZoneReq{Name: "yard", MaxSpeed: &speed})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *zone.MaxSpeed, test.ShouldEqual, speed)
	test.That(t, client.DeleteZone(ctx, "yard", nil), test.ShouldBeNil)

	got, err := client.NavigateToLocation(ctx, locName, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)
	got, err = client.NavigateToPoint(ctx, pos, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)
	got, err = client.NavigateRoute(ctx, navigation.NavigateRouteReq{Name: "loop", StartIndex: 1, Loop: true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)

	plan, err := client.PlanToPoint(ctx, navigation.PlanToPointReq{Destination: pos, Start: &pos})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan.Path, test.ShouldHaveLength, 2)
	test.That(t, plan.LengthM, test.ShouldEqual, 12)
	plan, err = client.PlanToLocation(ctx, navigation.PlanToLocationReq{Name: locName})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *plan.Location, test.ShouldEqual, locName)
	got, err = client.ExecutePlan(ctx, plan, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)

	got, err = client.GoToLocation(ctx, navigation.GoToLocationReq{Name: locName, Wait: &wait, LinearMPerSec: &speed})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)
	got, err = client.GoToPoint(ctx, navigation.GoToPointReq{Destination: pos})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)

	got, err = client.Suspend(ctx, "pause", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)
	got, err = client.Resume(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)
	got, err = client.Cancel(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)

	gotPos, err := client.GetPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gotPos.GeoPose.Location.Lat(), test.ShouldEqual, pos.GeoPose.Location.Lat())
	got, err = client.GetNavigationStatus(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.Localized, test.ShouldBeTrue)

	costmap, err := client.GetCostmap(ctx, navigation.CostmapReq{Layer: "local", Stride: 2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, costmap.Available, test.ShouldBeTrue)
	test.That(t, costmap.Data, test.ShouldResemble, []byte{1, 2})

	cmd, err := client.DoCommand(ctx, map[string]interface{}{"ping": "pong"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cmd["ping"], test.ShouldEqual, "pong")
	resourceStatus, err := client.Status(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resourceStatus["localized"], test.ShouldEqual, true)
}

func gpsPosition(lat, lng float64) navigation.Position {
	heading := 0.0
	return navigation.Position{GeoPose: &navigation.GeoPose{
		Location: geo.NewPoint(lat, lng),
		Heading:  &heading,
	}}
}

func TestClientDialFail(t *testing.T) {
	logger := logging.NewTestLogger(t)
	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	failingServer := grpc.NewServer()
	go failingServer.Serve(listener)
	defer failingServer.Stop()

	conn, err := viamgrpc.Dial(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()
	client, err := navigation.NewClientFromConn(context.Background(), conn, "", navigation.Named("nav"), logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = client.GetProperties(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	_ = servicepb.MapType_MAP_TYPE_GPS
}

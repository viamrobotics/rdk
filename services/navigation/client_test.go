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

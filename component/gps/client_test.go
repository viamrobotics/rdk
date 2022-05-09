package gps_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/gps/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	loc := geo.NewPoint(90, 1)
	alt := 50.5
	speed := 5.4
	rs := []interface{}{loc.Lat(), loc.Lng(), alt, speed}

	injectGPS := &inject.GPS{}
	injectGPS.ReadLocationFunc = func(ctx context.Context) (*geo.Point, error) { return loc, nil }
	injectGPS.ReadAltitudeFunc = func(ctx context.Context) (float64, error) { return alt, nil }
	injectGPS.ReadSpeedFunc = func(ctx context.Context) (float64, error) { return speed, nil }

	injectGPS2 := &inject.GPS{}
	injectGPS2.ReadLocationFunc = func(ctx context.Context) (*geo.Point, error) { return nil, errors.New("can't get location") }
	injectGPS2.ReadAltitudeFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get altitude") }
	injectGPS2.ReadSpeedFunc = func(ctx context.Context) (float64, error) { return 0, errors.New("can't get speed") }

	gpsSvc, err := subtype.New(map[resource.Name]interface{}{gps.Named(testGPSName): injectGPS, gps.Named(failGPSName): injectGPS2})
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(gps.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, gpsSvc)

	injectGPS.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, gpsSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = gps.NewClient(cancelCtx, testGPSName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("GPS client 1", func(t *testing.T) {
		// working
		gps1Client, err := gps.NewClient(context.Background(), testGPSName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		// Do
		resp, err := gps1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		loc1, err := gps1Client.ReadLocation(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, loc1, test.ShouldResemble, loc)

		alt1, err := gps1Client.ReadAltitude(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, alt1, test.ShouldAlmostEqual, alt)

		speed1, err := gps1Client.ReadSpeed(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed1, test.ShouldAlmostEqual, speed)

		rs1, err := gps1Client.(sensor.Sensor).GetReadings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		test.That(t, utils.TryClose(context.Background(), gps1Client), test.ShouldBeNil)
	})

	t.Run("GPS client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, failGPSName, logger)
		gps2Client, ok := client.(gps.GPS)
		test.That(t, ok, test.ShouldBeTrue)

		_, err = gps2Client.ReadLocation(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		_, err = gps2Client.ReadAltitude(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get altitude")

		_, err = gps2Client.ReadSpeed(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get speed")

		_, err = gps2Client.(sensor.Sensor).GetReadings(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		test.That(t, utils.TryClose(context.Background(), gps2Client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectGPS := &inject.GPS{}

	gpsSvc, err := subtype.New(map[resource.Name]interface{}{gps.Named(testGPSName): injectGPS})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterGPSServiceServer(gServer, gps.NewServer(gpsSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := gps.NewClient(ctx, testGPSName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := gps.NewClient(ctx, testGPSName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

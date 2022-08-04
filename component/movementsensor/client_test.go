package movementsensor_test

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
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/component/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/movementsensor/v1"
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

	injectMovementSensor := &inject.MovementSensor{}
	injectMovementSensor.GetPositionFunc = func(ctx context.Context) (*geo.Point, float64, float64, error) { return loc, alt, 0, nil }
	injectMovementSensor.GetLinearVelocityFunc = func(ctx context.Context) (r3.Vector, error) { return r3.Vector{0, speed, 0}, nil }

	injectMovementSensor2 := &inject.MovementSensor{}
	injectMovementSensor2.GetPositionFunc = func(ctx context.Context) (*geo.Point, float64, float64, error) { return nil, 0, 0, errors.New("can't get location") }
	injectMovementSensor2.GetLinearVelocityFunc = func(ctx context.Context) (r3.Vector, error) { return r3.Vector{}, errors.New("can't get linear velocity") }

	gpsSvc, err := subtype.New(map[resource.Name]interface{}{movementsensor.Named(testMovementSensorName): injectMovementSensor, movementsensor.Named(failMovementSensorName): injectMovementSensor2})
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(movementsensor.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, gpsSvc)

	injectMovementSensor.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, gpsSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("MovementSensor client 1", func(t *testing.T) {
		// working
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		gps1Client := movementsensor.NewClientFromConn(context.Background(), conn, testMovementSensorName, logger)

		// Do
		resp, err := gps1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		loc1, alt1, _, err := gps1Client.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, loc1, test.ShouldResemble, loc)
		test.That(t, alt1, test.ShouldAlmostEqual, alt)

		vel1, err := gps1Client.GetLinearVelocity(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vel1.Y, test.ShouldAlmostEqual, speed)

		rs1, err := gps1Client.GetReadings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		test.That(t, utils.TryClose(context.Background(), gps1Client), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("MovementSensor client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, failMovementSensorName, logger)
		gps2Client, ok := client.(movementsensor.MovementSensor)
		test.That(t, ok, test.ShouldBeTrue)

		_, _, _, err = gps2Client.GetPosition(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		_, err = gps2Client.GetLinearVelocity(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get linear velocity")

		_, err = gps2Client.GetAngularVelocity(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get angular velocity")

		_, err = gps2Client.(sensor.Sensor).GetReadings(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get location")

		test.That(t, utils.TryClose(context.Background(), gps2Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectMovementSensor := &inject.MovementSensor{}

	gpsSvc, err := subtype.New(map[resource.Name]interface{}{movementsensor.Named(testMovementSensorName): injectMovementSensor})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterMovementSensorServiceServer(gServer, movementsensor.NewServer(gpsSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := movementsensor.NewClientFromConn(ctx, conn1, testMovementSensorName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := movementsensor.NewClientFromConn(ctx, conn2, testMovementSensorName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

package sensor_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/component/sensor/v1"
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

	rs := map[string]interface{}{"a": 1.1, "b": 2.2}

	injectSensor := &inject.Sensor{}
	injectSensor.GetReadingsFunc = func(ctx context.Context) (map[string]interface{}, error) { return rs, nil }

	injectSensor2 := &inject.Sensor{}
	injectSensor2.GetReadingsFunc = func(ctx context.Context) (map[string]interface{}, error) {
		return nil, errors.New("can't get readings")
	}

	sensorSvc, err := subtype.New(
		(map[resource.Name]interface{}{sensor.Named(testSensorName): injectSensor, sensor.Named(failSensorName): injectSensor2}),
	)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(sensor.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, sensorSvc)

	injectSensor.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, sensorSvc)

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

	t.Run("Sensor client 1", func(t *testing.T) {
		// working
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		sensor1Client := sensor.NewClientFromConn(context.Background(), conn, testSensorName, logger)

		// DoCommand
		resp, err := sensor1Client.DoCommand(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		rs1, err := sensor1Client.GetReadings(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)

		test.That(t, utils.TryClose(context.Background(), sensor1Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("Sensor client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, failSensorName, logger)
		sensor2Client, ok := client.(sensor.Sensor)
		test.That(t, ok, test.ShouldBeTrue)

		_, err = sensor2Client.GetReadings(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get readings")

		test.That(t, utils.TryClose(context.Background(), sensor2Client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectSensor := &inject.Sensor{}
	sensor1 := "sensor1"

	sensorSvc, err := subtype.New(map[resource.Name]interface{}{sensor.Named(sensor1): injectSensor})
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterSensorServiceServer(gServer, sensor.NewServer(sensorSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)

	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := sensor.NewClientFromConn(ctx, conn1, testSensorName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := sensor.NewClientFromConn(ctx, conn2, testSensorName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

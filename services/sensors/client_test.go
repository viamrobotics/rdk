package sensors_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/sensors/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/movementsensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
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

	var extraOptions map[string]interface{}

	injectSensors := &inject.SensorsService{}
	ssMap := map[resource.Name]interface{}{
		sensors.Named(testSvcName1): injectSensors,
	}
	svc, err := subtype.New(ssMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(sensors.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working client
	t.Run("sensors client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		client := sensors.NewClientFromConn(context.Background(), conn, testSvcName1, logger)

		names := []resource.Name{movementsensor.Named("gps"), movementsensor.Named("imu")}
		injectSensors.SensorsFunc = func(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
			extraOptions = extra
			return names, nil
		}
		extra := map[string]interface{}{"foo": "Sensors"}
		sensorNames, err := client.Sensors(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, sensorNames, test.ShouldResemble, names)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		gReading := sensors.Readings{Name: movementsensor.Named("gps"), Readings: map[string]interface{}{"a": 4.5, "b": 5.6, "c": 6.7}}
		readings := []sensors.Readings{gReading}
		expected := map[resource.Name]interface{}{
			gReading.Name: gReading.Readings,
		}

		injectSensors.ReadingsFunc = func(
			ctx context.Context, sensors []resource.Name, extra map[string]interface{},
		) ([]sensors.Readings, error) {
			extraOptions = extra
			return readings, nil
		}
		extra = map[string]interface{}{"foo": "Readings"}
		readings, err = client.Readings(context.Background(), []resource.Name{}, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(readings), test.ShouldEqual, 1)
		observed := map[resource.Name]interface{}{
			readings[0].Name: readings[0].Readings,
		}
		test.That(t, observed, test.ShouldResemble, expected)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	// broken client
	t.Run("sensors client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testSvcName1, logger)
		client2, ok := client.(sensors.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("can't get sensors")
		injectSensors.SensorsFunc = func(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
			return nil, passedErr
		}

		_, err = client2.Sensors(context.Background(), map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())

		passedErr = errors.New("can't get readings")
		injectSensors.ReadingsFunc = func(
			ctx context.Context, sensors []resource.Name, extra map[string]interface{},
		) ([]sensors.Readings, error) {
			return nil, passedErr
		}
		_, err = client2.Readings(context.Background(), []resource.Name{}, map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())

		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectSensors := &inject.SensorsService{}
	ssMap := map[resource.Name]interface{}{
		sensors.Named(testSvcName1): injectSensors,
	}
	server, err := newServer(ssMap)
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterSensorsServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := sensors.NewClientFromConn(ctx, conn1, testSvcName1, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := sensors.NewClientFromConn(ctx, conn2, testSvcName1, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

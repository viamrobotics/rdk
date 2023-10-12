package sensors_test

import (
	"context"
	"net"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/movementsensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var testSvcName1 = sensors.Named("sen1")

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var extraOptions map[string]interface{}

	injectSensors := &inject.SensorsService{}
	ssMap := map[resource.Name]sensors.Service{
		testSvcName1: injectSensors,
	}
	svc, err := resource.NewAPIResourceCollection(sensors.API, ssMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[sensors.Service](sensors.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc), test.ShouldBeNil)

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

		client, err := sensors.NewClientFromConn(context.Background(), conn, "", testSvcName1, logger)
		test.That(t, err, test.ShouldBeNil)

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

		// DoCommand
		injectSensors.DoCommandFunc = testutils.EchoFunc
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	// broken client
	t.Run("sensors client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", testSvcName1, logger)
		test.That(t, err, test.ShouldBeNil)

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

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

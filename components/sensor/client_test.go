package sensor_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testSensorName    = "sensor1"
	failSensorName    = "sensor2"
	missingSensorName = "sensor4"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	rs := map[string]interface{}{"a": 1.1, "b": 2.2}

	var extraCap map[string]interface{}
	injectSensor := &inject.Sensor{}
	injectSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		extraCap = extra
		return rs, nil
	}

	injectSensor2 := &inject.Sensor{}
	injectSensor2.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return nil, errReadingsFailed
	}

	sensorSvc, err := resource.NewAPIResourceCollection(
		sensors.API,
		map[resource.Name]sensor.Sensor{sensor.Named(testSensorName): injectSensor, sensor.Named(failSensorName): injectSensor2},
	)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[sensor.Sensor](sensor.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, sensorSvc), test.ShouldBeNil)

	injectSensor.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("Sensor client 1", func(t *testing.T) {
		// working
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		sensor1Client, err := sensor.NewClientFromConn(context.Background(), conn, "", sensor.Named(testSensorName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := sensor1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		rs1, err := sensor1Client.Readings(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)
		test.That(t, extraCap, test.ShouldResemble, make(map[string]interface{}))

		// With extra params
		rs1, err = sensor1Client.Readings(context.Background(), map[string]interface{}{"foo": "bar"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rs1, test.ShouldResemble, rs)
		test.That(t, extraCap, test.ShouldResemble, map[string]interface{}{"foo": "bar"})

		test.That(t, sensor1Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("Sensor client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", sensor.Named(failSensorName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = client2.Readings(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errReadingsFailed.Error())

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

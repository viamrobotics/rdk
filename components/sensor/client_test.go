package sensor_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testSensorName    = "sensor1"
	failSensorName    = "sensor2"
	fakeSensorName    = "sensor3"
	missingSensorName = "sensor4"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
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
		return nil, errors.New("can't get readings")
	}

	sensorSvc, err := subtype.New(
		sensors.Subtype,
		map[resource.Name]resource.Resource{sensor.Named(testSensorName): injectSensor, sensor.Named(failSensorName): injectSensor2},
	)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok := registry.ResourceSubtypeLookup(sensor.Subtype)
	test.That(t, ok, test.ShouldBeTrue)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, sensorSvc)

	injectSensor.DoFunc = testutils.EchoFunc
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
		sensor1Client, err := sensor.NewClientFromConn(context.Background(), conn, sensor.Named(testSensorName), logger)
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
		client, err := resourceSubtype.RPCClient(context.Background(), conn, sensor.Named(failSensorName), logger)
		test.That(t, err, test.ShouldBeNil)
		sensor2Client, ok := client.(sensor.Sensor)
		test.That(t, ok, test.ShouldBeTrue)

		_, err = sensor2Client.Readings(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't get readings")

		test.That(t, sensor2Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

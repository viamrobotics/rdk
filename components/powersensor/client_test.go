// package powersensor_test contains tests for powersensor
package powersensor_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/powersensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	testVolts := 4.8
	testAmps := 3.8
	testWatts := 2.8
	testIsAC := false

	workingPowerSensor := &inject.PowerSensor{}
	failingPowerSensor := &inject.PowerSensor{}

	workingPowerSensor.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return testVolts, testIsAC, nil
	}

	workingPowerSensor.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return testAmps, testIsAC, nil
	}

	workingPowerSensor.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return testWatts, nil
	}

	failingPowerSensor.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 0, false, errVoltageFailed
	}

	failingPowerSensor.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 0, false, errCurrentFailed
	}

	failingPowerSensor.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, errPowerFailed
	}

	resourceMap := map[resource.Name]powersensor.PowerSensor{
		motor.Named(workingPowerSensorName): workingPowerSensor,
		motor.Named(failingPowerSensorName): failingPowerSensor,
	}
	powersensorSvc, err := resource.NewAPIResourceCollection(powersensor.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[powersensor.PowerSensor](powersensor.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, powersensorSvc), test.ShouldBeNil)

	workingPowerSensor.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing client
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client, err := powersensor.NewClientFromConn(context.Background(), conn, "", motor.Named(workingPowerSensorName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests with working power sensor", func(t *testing.T) {
		// DoCommand
		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		volts, isAC, err := client.Voltage(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, volts, test.ShouldEqual, testVolts)
		test.That(t, isAC, test.ShouldEqual, testIsAC)

		amps, isAC, err := client.Current(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, amps, test.ShouldEqual, testAmps)
		test.That(t, isAC, test.ShouldEqual, testIsAC)

		watts, err := client.Power(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, watts, test.ShouldEqual, testWatts)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	conn, err = viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client, err = powersensor.NewClientFromConn(context.Background(), conn, "", powersensor.Named(failingPowerSensorName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests with failing power sensor", func(t *testing.T) {
		volts, isAC, err := client.Voltage(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errVoltageFailed.Error())
		test.That(t, volts, test.ShouldEqual, 0)
		test.That(t, isAC, test.ShouldEqual, false)

		amps, isAC, err := client.Current(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCurrentFailed.Error())
		test.That(t, amps, test.ShouldEqual, 0)
		test.That(t, isAC, test.ShouldEqual, false)

		watts, err := client.Power(context.Background(), make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPowerFailed.Error())
		test.That(t, watts, test.ShouldEqual, 0)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

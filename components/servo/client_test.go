package servo_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/servo"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testServoName = "servo1"
	failServoName = "servo2"
	fakeServoName = "servo3"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var actualExtra map[string]interface{}

	workingServo := &inject.Servo{}
	failingServo := &inject.Servo{}

	workingServo.MoveFunc = func(ctx context.Context, angle uint32, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		actualExtra = extra
		return 20, nil
	}
	workingServo.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}

	failingServo.MoveFunc = func(ctx context.Context, angle uint32, extra map[string]interface{}) error {
		return errMoveFailed
	}
	failingServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 0, errPositionUnreadable
	}
	failingServo.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopFailed
	}

	resourceMap := map[resource.Name]servo.Servo{
		servo.Named(testServoName): workingServo,
		servo.Named(failServoName): failingServo,
	}
	servoSvc, err := resource.NewAPIResourceCollection(servo.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[servo.Servo](servo.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, servoSvc), test.ShouldBeNil)

	workingServo.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("client tests for working servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingServoClient, err := servo.NewClientFromConn(context.Background(), conn, "", servo.Named(testServoName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := workingServoClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		err = workingServoClient.Move(context.Background(), 20, map[string]interface{}{"foo": "Move"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "Move"})

		currentDeg, err := workingServoClient.Position(context.Background(), map[string]interface{}{"foo": "Position"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentDeg, test.ShouldEqual, 20)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "Position"})

		test.That(t, workingServoClient.Stop(context.Background(), map[string]interface{}{"foo": "Stop"}), test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "Stop"})

		test.That(t, workingServoClient.Close(context.Background()), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests for failing servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingServoClient, err := servo.NewClientFromConn(context.Background(), conn, "", servo.Named(failServoName), logger)
		test.That(t, err, test.ShouldBeNil)

		err = failingServoClient.Move(context.Background(), 20, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errMoveFailed.Error())

		_, err = failingServoClient.Position(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errPositionUnreadable.Error())

		err = failingServoClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStopFailed.Error())

		test.That(t, failingServoClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceAPI.RPCClient(context.Background(), conn, "", servo.Named(testServoName), logger)
		test.That(t, err, test.ShouldBeNil)

		err = client.Move(context.Background(), 20, nil)
		test.That(t, err, test.ShouldBeNil)

		currentDeg, err := client.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentDeg, test.ShouldEqual, 20)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

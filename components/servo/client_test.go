package servo_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	componentpb "go.viam.com/api/component/servo/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/servo"
	viamgrpc "go.viam.com/rdk/grpc"
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

	var actualExtra map[string]interface{}

	workingServo := &inject.Servo{}
	failingServo := &inject.Servo{}

	workingServo.MoveFunc = func(ctx context.Context, angle uint8, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint8, error) {
		actualExtra = extra
		return 20, nil
	}
	workingServo.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}

	failingServo.MoveFunc = func(ctx context.Context, angle uint8, extra map[string]interface{}) error {
		return errors.New("move failed")
	}
	failingServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint8, error) {
		return 0, errors.New("current angle not readable")
	}
	failingServo.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New("no stop")
	}

	resourceMap := map[resource.Name]interface{}{
		servo.Named(testServoName): workingServo,
		servo.Named(failServoName): failingServo,
	}
	servoSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(servo.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, servoSvc)

	workingServo.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, servoSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("client tests for working servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingServoClient := servo.NewClientFromConn(context.Background(), conn, testServoName, logger)

		// DoCommand
		resp, err := workingServoClient.DoCommand(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		err = workingServoClient.Move(context.Background(), 20, map[string]interface{}{"foo": "Move"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "Move"})

		currentDeg, err := workingServoClient.Position(context.Background(), map[string]interface{}{"foo": "Position"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentDeg, test.ShouldEqual, 20)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "Position"})

		test.That(t, workingServoClient.Stop(context.Background(), map[string]interface{}{"foo": "Stop"}), test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "Stop"})

		test.That(t, utils.TryClose(context.Background(), workingServoClient), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests for failing servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingServoClient := servo.NewClientFromConn(context.Background(), conn, failServoName, logger)

		err = failingServoClient.Move(context.Background(), 20, nil)
		test.That(t, err, test.ShouldNotBeNil)

		_, err = failingServoClient.Position(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingServoClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no stop")

		test.That(t, utils.TryClose(context.Background(), failingServoClient), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testServoName, logger)
		workingServoDialedClient, ok := client.(servo.Servo)
		test.That(t, ok, test.ShouldBeTrue)

		err = workingServoDialedClient.Move(context.Background(), 20, nil)
		test.That(t, err, test.ShouldBeNil)

		currentDeg, err := workingServoDialedClient.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentDeg, test.ShouldEqual, 20)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectServo := &inject.Servo{}

	servoSvc, err := subtype.New(map[resource.Name]interface{}{servo.Named(testServoName): injectServo})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterServoServiceServer(gServer, servo.NewServer(servoSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := servo.NewClientFromConn(ctx, conn1, testServoName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := servo.NewClientFromConn(ctx, conn2, testServoName, logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

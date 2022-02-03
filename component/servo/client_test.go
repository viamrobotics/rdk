package servo_test

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

	"go.viam.com/rdk/component/servo"
	viamgrpc "go.viam.com/rdk/grpc"
	componentpb "go.viam.com/rdk/proto/api/component/v1"
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

	workingServo := &inject.Servo{}
	failingServo := &inject.Servo{}

	workingServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		return nil
	}
	workingServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 20, nil
	}

	failingServo.MoveFunc = func(ctx context.Context, angle uint8) error {
		return errors.New("move failed")
	}
	failingServo.CurrentFunc = func(ctx context.Context) (uint8, error) {
		return 0, errors.New("current angle not readable")
	}

	resourceMap := map[resource.Name]interface{}{
		servo.Named(testServoName): workingServo,
		servo.Named(failServoName): failingServo,
	}
	servoSvc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(servo.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, servoSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = servo.NewClient(cancelCtx, testServoName, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("client tests for working servo", func(t *testing.T) {
		workingServoClient, err := servo.NewClient(context.Background(), testServoName, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)

		err = workingServoClient.Move(context.Background(), 20)
		test.That(t, err, test.ShouldBeNil)

		currentDeg, err := workingServoClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentDeg, test.ShouldEqual, 20)

		test.That(t, utils.TryClose(context.Background(), workingServoClient), test.ShouldBeNil)
	})

	t.Run("client tests for failing servo", func(t *testing.T) {
		failingServoClient, err := servo.NewClient(context.Background(), failServoName, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)

		err = failingServoClient.Move(context.Background(), 20)
		test.That(t, err, test.ShouldNotBeNil)

		_, err = failingServoClient.GetPosition(context.Background())
		test.That(t, err, test.ShouldNotBeNil)

		test.That(t, utils.TryClose(context.Background(), failingServoClient), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working servo", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testServoName, logger)
		workingServoDialedClient, ok := client.(servo.Servo)
		test.That(t, ok, test.ShouldBeTrue)

		err = workingServoDialedClient.Move(context.Background(), 20)
		test.That(t, err, test.ShouldBeNil)

		currentDeg, err := workingServoDialedClient.GetPosition(context.Background())
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

	servoSvc, err := subtype.New((map[resource.Name]interface{}{servo.Named(testServoName): injectServo}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterServoServiceServer(gServer, servo.NewServer(servoSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := servo.NewClient(ctx, testServoName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := servo.NewClient(ctx, testServoName, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

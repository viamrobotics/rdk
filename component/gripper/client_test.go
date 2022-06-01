package gripper_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gripper"
	viamgrpc "go.viam.com/rdk/grpc"
	componentpb "go.viam.com/rdk/proto/api/component/gripper/v1"
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

	var gripperOpen string

	grabbed1 := true
	injectGripper := &inject.Gripper{}
	injectGripper.OpenFunc = func(ctx context.Context) error {
		gripperOpen = testGripperName
		return nil
	}
	injectGripper.GrabFunc = func(ctx context.Context) (bool, error) { return grabbed1, nil }

	injectGripper2 := &inject.Gripper{}
	injectGripper2.OpenFunc = func(ctx context.Context) error {
		gripperOpen = failGripperName
		return errors.New("can't open")
	}
	injectGripper2.GrabFunc = func(ctx context.Context) (bool, error) { return false, errors.New("can't grab") }

	gripperSvc, err := subtype.New(
		map[resource.Name]interface{}{gripper.Named(testGripperName): injectGripper, gripper.Named(failGripperName): injectGripper2})
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(gripper.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, gripperSvc)

	injectGripper.DoFunc = generic.EchoFunc
	generic.RegisterService(rpcServer, gripperSvc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = gripper.NewClient(cancelCtx, testGripperName, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("gripper client 1", func(t *testing.T) {
		gripper1Client, err := gripper.NewClient(
			context.Background(),
			testGripperName,
			listener1.Addr().String(),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		// Do
		resp, err := gripper1Client.Do(context.Background(), generic.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, generic.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, generic.TestCommand["data"])

		err = gripper1Client.Open(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gripperOpen, test.ShouldEqual, testGripperName)

		grabbed, err := gripper1Client.Grab(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, grabbed, test.ShouldEqual, grabbed1)

		test.That(t, utils.TryClose(context.Background(), gripper1Client), test.ShouldBeNil)
	})

	t.Run("gripper client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, failGripperName, logger)
		gripper2Client, ok := client.(gripper.Gripper)
		test.That(t, ok, test.ShouldBeTrue)

		err = gripper2Client.Open(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't open")
		test.That(t, gripperOpen, test.ShouldEqual, failGripperName)

		grabbed, err := gripper2Client.Grab(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't grab")
		test.That(t, grabbed, test.ShouldEqual, false)

		test.That(t, utils.TryClose(context.Background(), gripper2Client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectGripper := &inject.Gripper{}

	gripperSvc, err := subtype.New(map[resource.Name]interface{}{gripper.Named(testGripperName): injectGripper})
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGripperServiceServer(gServer, gripper.NewServer(gripperSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := gripper.NewClient(ctx, testGripperName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := gripper.NewClient(ctx, testGripperName, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

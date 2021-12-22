package gripper_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"go.viam.com/utils"

	"go.viam.com/core/component/gripper"
	componentpb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/subtype"
	"go.viam.com/core/testutils"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/core/grpc"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeNil)
	gServer1 := grpc.NewServer()

	var gripperOpen string

	gripper1 := "gripper1"
	grabbed1 := true
	injectGripper := &inject.Gripper{}
	injectGripper.OpenFunc = func(ctx context.Context) error {
		gripperOpen = gripper1
		return nil
	}
	injectGripper.GrabFunc = func(ctx context.Context) (bool, error) { return grabbed1, nil }

	gripper2 := "gripper2"
	injectGripper2 := &inject.Gripper{}
	injectGripper2.OpenFunc = func(ctx context.Context) error {
		gripperOpen = gripper2
		return errors.New("can't open")
	}
	injectGripper2.GrabFunc = func(ctx context.Context) (bool, error) { return false, errors.New("can't grab") }

	gripperSvc, err := subtype.New(
		(map[resource.Name]interface{}{gripper.Named(gripper1): injectGripper, gripper.Named(gripper2): injectGripper2}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGripperServiceServer(gServer1, gripper.NewServer(gripperSvc))

	go gServer1.Serve(listener1)
	defer gServer1.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = gripper.NewClient(cancelCtx, gripper1, listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	gripper1Client, err := gripper.NewClient(context.Background(), gripper1, listener1.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)

	t.Run("gripper client 1", func(t *testing.T) {
		err := gripper1Client.Open(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gripperOpen, test.ShouldEqual, gripper1)

		grabbed, err := gripper1Client.Grab(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, grabbed, test.ShouldEqual, grabbed1)
	})

	t.Run("gripper client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger, rpc.WithInsecure())
		test.That(t, err, test.ShouldBeNil)
		gripper2Client := gripper.NewClientFromConn(conn, gripper2, logger)
		test.That(t, err, test.ShouldBeNil)

		err = gripper2Client.Open(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't open")
		test.That(t, gripperOpen, test.ShouldEqual, gripper2)

		grabbed, err := gripper2Client.Grab(context.Background())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "can't grab")
		test.That(t, grabbed, test.ShouldEqual, false)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, utils.TryClose(gripper1Client), test.ShouldBeNil)
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()
	injectGripper := &inject.Gripper{}
	gripper1 := "gripper1"

	gripperSvc, err := subtype.New((map[resource.Name]interface{}{gripper.Named(gripper1): injectGripper}))
	test.That(t, err, test.ShouldBeNil)
	componentpb.RegisterGripperServiceServer(gServer, gripper.NewServer(gripperSvc))

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := gripper.NewClient(ctx, gripper1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	client2, err := gripper.NewClient(ctx, gripper1, listener.Addr().String(), logger, rpc.WithInsecure())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.DialCalled, test.ShouldEqual, 2)

	err = utils.TryClose(client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(client2)
	test.That(t, err, test.ShouldBeNil)
}

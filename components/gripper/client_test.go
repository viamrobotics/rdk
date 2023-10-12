package gripper_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/gripper"
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
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var gripperOpen string
	var extraOptions map[string]interface{}

	grabbed1 := true
	injectGripper := &inject.Gripper{}
	injectGripper.OpenFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		gripperOpen = testGripperName
		return nil
	}
	injectGripper.GrabFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		extraOptions = extra
		return grabbed1, nil
	}
	injectGripper.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		return nil
	}

	injectGripper2 := &inject.Gripper{}
	injectGripper2.OpenFunc = func(ctx context.Context, extra map[string]interface{}) error {
		gripperOpen = failGripperName
		return errCantOpen
	}
	injectGripper2.GrabFunc = func(ctx context.Context, extra map[string]interface{}) (bool, error) {
		return false, errCantGrab
	}
	injectGripper2.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errStopUnimplemented
	}

	gripperSvc, err := resource.NewAPIResourceCollection(
		gripper.API,
		map[resource.Name]gripper.Gripper{gripper.Named(testGripperName): injectGripper, gripper.Named(failGripperName): injectGripper2})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[gripper.Gripper](gripper.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, gripperSvc), test.ShouldBeNil)

	injectGripper.DoFunc = testutils.EchoFunc

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

	// working
	t.Run("gripper client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		gripper1Client, err := gripper.NewClientFromConn(context.Background(), conn, "", gripper.Named(testGripperName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := gripper1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		extra := map[string]interface{}{"foo": "Open"}
		err = gripper1Client.Open(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, gripperOpen, test.ShouldEqual, testGripperName)

		extra = map[string]interface{}{"foo": "Grab"}
		grabbed, err := gripper1Client.Grab(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, grabbed, test.ShouldEqual, grabbed1)

		extra = map[string]interface{}{"foo": "Stop"}
		test.That(t, gripper1Client.Stop(context.Background(), extra), test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		test.That(t, gripper1Client.Close(context.Background()), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("gripper client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", gripper.Named(failGripperName), logger)
		test.That(t, err, test.ShouldBeNil)

		extra := map[string]interface{}{}
		err = client2.Open(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantOpen.Error())
		test.That(t, gripperOpen, test.ShouldEqual, failGripperName)

		grabbed, err := client2.Grab(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGrab.Error())
		test.That(t, grabbed, test.ShouldEqual, false)

		err = client2.Stop(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errStopUnimplemented.Error())

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

package button_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/button"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testButtonName    = "button1"
	testButtonName2   = "button2"
	failButtonName    = "button3"
	missingButtonName = "button4"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var buttonPushed string
	var extraOptions map[string]interface{}

	injectButton := inject.NewButton(testButtonName)
	injectButton.PushFunc = func(ctx context.Context, extra map[string]interface{}) error {
		extraOptions = extra
		buttonPushed = testButtonName
		return nil
	}

	injectButton2 := inject.NewButton(failButtonName)
	injectButton2.PushFunc = func(ctx context.Context, extra map[string]interface{}) error {
		buttonPushed = failButtonName
		return errCantPush
	}

	buttonSvc, err := resource.NewAPIResourceCollection(
		button.API,
		map[resource.Name]button.Button{button.Named(testButtonName): injectButton, button.Named(failButtonName): injectButton2})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[button.Button](button.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, buttonSvc, logger), test.ShouldBeNil)

	injectButton.DoFunc = testutils.EchoFunc

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
	t.Run("button client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		button1Client, err := button.NewClientFromConn(context.Background(), conn, "", button.Named(testButtonName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := button1Client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		extra := map[string]interface{}{"foo": "Push"}
		err = button1Client.Push(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, buttonPushed, test.ShouldEqual, testButtonName)

		test.That(t, button1Client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("button client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", button.Named(failButtonName), logger)
		test.That(t, err, test.ShouldBeNil)

		extra := map[string]interface{}{}
		err = client2.Push(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantPush.Error())
		test.That(t, buttonPushed, test.ShouldEqual, failButtonName)

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

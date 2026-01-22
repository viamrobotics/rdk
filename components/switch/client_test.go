package toggleswitch_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	toggleswitch "go.viam.com/rdk/components/switch"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testSwitchName     = "switch1"
	failSwitchName     = "switch2"
	missingSwitchName  = "missing"
	mismatchSwitchName = "mismatch"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var switchName string
	var extraOptions map[string]interface{}

	injectSwitch := inject.NewSwitch(testSwitchName)
	injectSwitch.SetPositionFunc = func(ctx context.Context, position uint32, extra map[string]interface{}) error {
		extraOptions = extra
		switchName = testSwitchName
		return nil
	}
	injectSwitch.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		extraOptions = extra
		switchName = testSwitchName
		return 0, nil
	}
	injectSwitch.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
		extraOptions = extra
		switchName = testSwitchName
		return 2, []string{"position 1", "position 2"}, nil
	}
	injectSwitch.DoFunc = testutils.EchoFunc

	injectSwitch2 := inject.NewSwitch(failSwitchName)
	injectSwitch2.SetPositionFunc = func(ctx context.Context, position uint32, extra map[string]interface{}) error {
		switchName = failSwitchName
		return errCantSetPosition
	}
	injectSwitch2.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		switchName = failSwitchName
		return 0, errCantGetPosition
	}
	injectSwitch2.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
		switchName = failSwitchName
		return 0, nil, errCantGetNumberOfPositions
	}
	injectSwitch2.DoFunc = testutils.EchoFunc

	injectSwitch3 := inject.NewSwitch(mismatchSwitchName)
	injectSwitch3.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
		switchName = mismatchSwitchName
		return 1, []string{"A", "B"}, nil
	}

	switchSvc, err := resource.NewAPIResourceCollection(
		toggleswitch.API,
		map[resource.Name]toggleswitch.Switch{
			toggleswitch.Named(testSwitchName):     injectSwitch,
			toggleswitch.Named(failSwitchName):     injectSwitch2,
			toggleswitch.Named(mismatchSwitchName): injectSwitch3,
		})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[toggleswitch.Switch](toggleswitch.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, switchSvc, logger), test.ShouldBeNil)

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
	t.Run("switch client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client1, err := toggleswitch.NewClientFromConn(context.Background(), conn, "", toggleswitch.Named(testSwitchName), logger)
		test.That(t, err, test.ShouldBeNil)

		// DoCommand
		resp, err := client1.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		extra := map[string]interface{}{"foo": "SetPosition"}
		err = client1.SetPosition(context.Background(), 0, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, switchName, test.ShouldEqual, testSwitchName)

		extra = map[string]interface{}{"foo": "GetPosition"}
		pos, err := client1.GetPosition(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, pos, test.ShouldEqual, 0)

		extra = map[string]interface{}{"foo": "GetNumberOfPositions"}
		count, labels, err := client1.GetNumberOfPositions(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, count, test.ShouldEqual, 2)
		test.That(t, labels, test.ShouldResemble, []string{"position 1", "position 2"})

		test.That(t, client1.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("switch client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", toggleswitch.Named(failSwitchName), logger)
		test.That(t, err, test.ShouldBeNil)

		extra := map[string]interface{}{}
		err = client2.SetPosition(context.Background(), 0, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantSetPosition.Error())
		test.That(t, switchName, test.ShouldEqual, failSwitchName)

		_, err = client2.GetPosition(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGetPosition.Error())

		_, _, err = client2.GetNumberOfPositions(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGetNumberOfPositions.Error())

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("mismatch client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		mismatchClient, err := resourceAPI.RPCClient(context.Background(), conn, "", toggleswitch.Named(mismatchSwitchName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, _, err = mismatchClient.GetNumberOfPositions(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errLabelCountMismatch.Error())

		test.That(t, mismatchClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

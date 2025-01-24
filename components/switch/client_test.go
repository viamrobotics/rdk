package switch_component_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	switch_component "go.viam.com/rdk/components/switch"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testSwitchName    = "switch1"
	failSwitchName    = "switch2"
	missingSwitchName = "missing"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var switchName string
	var extraOptions map[string]interface{}

	injectSwitch := &inject.Switch{}
	injectSwitch.SetPositionFunc = func(ctx context.Context, position uint32, extra map[string]interface{}) error {
		extraOptions = extra
		switchName = testSwitchName
		return nil
	}
	injectSwitch.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		extraOptions = extra
		return 0, nil
	}
	injectSwitch.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
		extraOptions = extra
		return 2, nil
	}

	injectSwitch2 := &inject.Switch{}
	injectSwitch2.SetPositionFunc = func(ctx context.Context, position uint32, extra map[string]interface{}) error {
		switchName = failSwitchName
		return errCantSetPosition
	}
	injectSwitch2.GetPositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 0, errCantGetPosition
	}
	injectSwitch2.GetNumberOfPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (int, error) {
		return 0, errCantGetNumberOfPositions
	}

	switchSvc, err := resource.NewAPIResourceCollection(
		switch_component.API,
		map[resource.Name]switch_component.Switch{
			switch_component.Named(testSwitchName): injectSwitch,
			switch_component.Named(failSwitchName): injectSwitch2,
		})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[switch_component.Switch](switch_component.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, switchSvc), test.ShouldBeNil)

	injectSwitch.DoFunc = testutils.EchoFunc

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
		client1, err := switch_component.NewClientFromConn(context.Background(), conn, "", switch_component.Named(testSwitchName), logger)
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
		count, err := client1.GetNumberOfPositions(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, count, test.ShouldEqual, 2)

		test.That(t, client1.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("switch client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client2, err := resourceAPI.RPCClient(context.Background(), conn, "", switch_component.Named(failSwitchName), logger)
		test.That(t, err, test.ShouldBeNil)

		extra := map[string]interface{}{}
		err = client2.SetPosition(context.Background(), 0, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantSetPosition.Error())
		test.That(t, switchName, test.ShouldEqual, failSwitchName)

		_, err = client2.GetPosition(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGetPosition.Error())

		_, err = client2.GetNumberOfPositions(context.Background(), extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errCantGetNumberOfPositions.Error())

		test.That(t, client2.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

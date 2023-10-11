package generic_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	testGenericName = "gen1"
	failGenericName = "gen2"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	workingGeneric := &inject.Generic{}
	failingGeneric := &inject.Generic{}

	workingGeneric.DoFunc = testutils.EchoFunc
	failingGeneric.DoFunc = func(
		ctx context.Context,
		cmd map[string]interface{},
	) (
		map[string]interface{},
		error,
	) {
		return nil, errDoFailed
	}

	resourceMap := map[resource.Name]resource.Resource{
		generic.Named(testGenericName): workingGeneric,
		generic.Named(failGenericName): failingGeneric,
	}
	genericSvc, err := resource.NewAPIResourceCollection(generic.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[resource.Resource](generic.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, genericSvc), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("client tests for working generic", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingGenericClient, err := generic.NewClientFromConn(context.Background(), conn, "", generic.Named(testGenericName), logger)
		test.That(t, err, test.ShouldBeNil)

		resp, err := workingGenericClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, workingGenericClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests for failing generic", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingGenericClient, err := generic.NewClientFromConn(context.Background(), conn, "", generic.Named(failGenericName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = failingGenericClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errDoFailed.Error())

		test.That(t, failingGenericClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working generic", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceAPI.RPCClient(context.Background(), conn, "", generic.Named(testGenericName), logger)
		test.That(t, err, test.ShouldBeNil)

		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

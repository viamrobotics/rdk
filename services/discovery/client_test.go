package discovery_test

import (
	"context"
	"net"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	testComponents := []resource.Config{createTestComponent("component-1"), createTestComponent("component-2")}

	workingDiscovery := inject.NewDiscoveryService(testDiscoveryName)
	workingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
		return testComponents, nil
	}
	workingDiscovery.DoFunc = testutils.EchoFunc

	failingDiscovery := inject.NewDiscoveryService(failDiscoveryName)
	failingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
		return nil, errDiscoverFailed
	}
	failingDiscovery.DoFunc = func(
		ctx context.Context,
		cmd map[string]interface{},
	) (
		map[string]interface{},
		error,
	) {
		return nil, errDoFailed
	}

	resourceMap := map[resource.Name]discovery.Service{
		discovery.Named(testDiscoveryName): workingDiscovery,
		discovery.Named(failDiscoveryName): failingDiscovery,
	}
	discoverySvc, err := resource.NewAPIResourceCollection(discovery.API, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[discovery.Service](discovery.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, discoverySvc), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})

	t.Run("client tests for working discovery", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingDiscoveryClient, err := discovery.NewClientFromConn(context.Background(), conn, "", discovery.Named(testDiscoveryName), logger)
		test.That(t, err, test.ShouldBeNil)

		respDis, err := workingDiscoveryClient.DiscoverResources(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(respDis), test.ShouldEqual, len(testComponents))
		for index, actual := range respDis {
			expected := testComponents[index]
			validateComponent(t, actual, expected)
		}

		resp, err := workingDiscoveryClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, workingDiscoveryClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests for failing discovery", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingDiscoveryClient, err := discovery.NewClientFromConn(context.Background(), conn, "", discovery.Named(failDiscoveryName), logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = failingDiscoveryClient.DiscoverResources(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errDiscoverFailed.Error())

		_, err = failingDiscoveryClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errDoFailed.Error())

		test.That(t, failingDiscoveryClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("client tests for failing discovery due to nil response", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingDiscoveryClient, err := discovery.NewClientFromConn(context.Background(), conn, "", discovery.Named(failDiscoveryName), logger)
		test.That(t, err, test.ShouldBeNil)

		failingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
			return nil, nil
		}
		_, err = failingDiscoveryClient.DiscoverResources(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, discovery.ErrNilResponse.Error())

		test.That(t, failingDiscoveryClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working discovery", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := resourceAPI.RPCClient(context.Background(), conn, "", discovery.Named(testDiscoveryName), logger)
		test.That(t, err, test.ShouldBeNil)

		resp, err := client.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

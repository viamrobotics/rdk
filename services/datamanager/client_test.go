package datamanager_test

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

	viamgrpc "go.viam.com/rdk/grpc"
	servicepb "go.viam.com/rdk/proto/api/service/datamanager/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
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

	injectMS := &inject.DataManagerService{}
	resourceMap := map[resource.Name]interface{}{
		datamanager.Name: injectMS,
	}
	svc, err := subtype.New(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(datamanager.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)
	test.That(t, err, test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	// failing
	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		client, err := datamanager.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	// working
	t.Run("datamanager client 1", func(t *testing.T) {
		client, err := datamanager.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		injectMS.SyncFunc = func(
			ctx context.Context,
		) error {
			return nil
		}
		err = client.Sync(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	// broken
	t.Run("datamanager client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		client2, ok := client.(datamanager.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("fake syncDataCaptureFiles error")
		injectMS.SyncFunc = func(
			ctx context.Context,
		) error {
			return passedErr
		}

		err = client2.Sync(context.Background())
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectMS := &inject.DataManagerService{}
	resourceMap := map[resource.Name]interface{}{
		datamanager.Name: injectMS,
	}
	server, err := newServer(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterDataManagerServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := datamanager.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := datamanager.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}

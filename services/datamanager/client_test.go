package datamanager_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/datamanager/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const testDataManagerServiceName = "DataManager1"

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	var extraOptions map[string]interface{}

	injectDS := &inject.DataManagerService{}
	resourceMap := map[resource.Name]interface{}{
		datamanager.Named(testDataManagerServiceName): injectDS,
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
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	// working
	t.Run("datamanager client 1", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := datamanager.NewClientFromConn(context.Background(), conn, testDataManagerServiceName, logger)

		injectDS.SyncFunc = func(ctx context.Context, extra map[string]interface{}) error {
			extraOptions = extra
			return nil
		}
		extra := map[string]interface{}{"foo": "Sync"}
		err = client.Sync(context.Background(), extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	// broken
	t.Run("datamanager client 2", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, testDataManagerServiceName, logger)
		client2, ok := client.(datamanager.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("fake sync error")
		injectDS.SyncFunc = func(ctx context.Context, extra map[string]interface{}) error {
			return passedErr
		}

		err = client2.Sync(context.Background(), map[string]interface{}{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectDS := &inject.DataManagerService{}
	resourceMap := map[resource.Name]interface{}{
		datamanager.Named(testDataManagerServiceName): injectDS,
	}
	server, err := newServer(resourceMap)
	test.That(t, err, test.ShouldBeNil)
	servicepb.RegisterDataManagerServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	conn1, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client1 := datamanager.NewClientFromConn(context.Background(), conn1, "", logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	conn2, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	client2 := datamanager.NewClientFromConn(context.Background(), conn2, "", logger)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, conn1.Close(), test.ShouldBeNil)
	test.That(t, conn2.Close(), test.ShouldBeNil)
}

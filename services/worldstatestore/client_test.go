package worldstatestore_test

import (
	"context"
	"net"
	"testing"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/worldstatestore"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	srv := &inject.WorldStateStoreService{}
	srv.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
		return [][]byte{[]byte("uuid1"), []byte("uuid2")}, nil
	}
	srv.GetTransformFunc = func(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
		return &commonpb.Transform{
			ReferenceFrame: "test-frame",
			Uuid:           uuid,
		}, nil
	}
	srv.StreamTransformChangesFunc = func(ctx context.Context, extra map[string]any) (*worldstatestore.TransformChangeStream, error) {
		changesChan := make(chan worldstatestore.TransformChange, 1)
		changesChan <- worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform: &commonpb.Transform{
				ReferenceFrame: "test-frame",
				Uuid:           []byte("test-uuid"),
			},
		}
		close(changesChan)
		return worldstatestore.NewTransformChangeStreamFromChannel(ctx, changesChan), nil
	}
	srv.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
		return cmd, nil
	}

	m := map[resource.Name]worldstatestore.Service{
		worldstatestore.Named(testWorldStateStoreServiceName): srv,
	}
	svc, err := resource.NewAPIResourceCollection(worldstatestore.API, m)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[worldstatestore.Service](worldstatestore.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc, logger), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("ListUUIDs", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		uuids, err := client.ListUUIDs(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(uuids), test.ShouldEqual, 2)
		test.That(t, uuids[0], test.ShouldResemble, []byte("uuid1"))
		test.That(t, uuids[1], test.ShouldResemble, []byte("uuid2"))

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("GetTransform", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		transform, err := client.GetTransform(context.Background(), []byte("test-uuid"), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, transform.ReferenceFrame, test.ShouldEqual, "test-frame")
		test.That(t, transform.Uuid, test.ShouldResemble, []byte("test-uuid"))

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("GetTransform returns ErrNilResponse when not found", func(t *testing.T) {
		srv.GetTransformFunc = func(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
			return nil, nil
		}

		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		obj, err := client.GetTransform(context.Background(), []byte("missing-uuid"), nil)
		test.That(t, err, test.ShouldEqual, worldstatestore.ErrNilResponse)
		test.That(t, obj, test.ShouldBeNil)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("StreamTransformChanges", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		stream, err := client.StreamTransformChanges(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		change, err := stream.Next()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, change.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
		test.That(t, change.Transform.ReferenceFrame, test.ShouldEqual, "test-frame")
		test.That(t, change.Transform.Uuid, test.ShouldResemble, []byte("test-uuid"))

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("DoCommand", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		cmd := map[string]interface{}{"test": "command"}
		resp, err := client.DoCommand(context.Background(), cmd)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, cmd)

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

func TestClientFailures(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	srv := &inject.WorldStateStoreService{}
	expectedErr := errors.New("fake error")
	srv.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
		return nil, expectedErr
	}
	srv.GetTransformFunc = func(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
		return nil, expectedErr
	}
	srv.StreamTransformChangesFunc = func(ctx context.Context, extra map[string]any) (*worldstatestore.TransformChangeStream, error) {
		return nil, expectedErr
	}

	m := map[resource.Name]worldstatestore.Service{
		worldstatestore.Named(testWorldStateStoreServiceName): srv,
	}
	svc, err := resource.NewAPIResourceCollection(worldstatestore.API, m)
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[worldstatestore.Service](worldstatestore.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, svc, logger), test.ShouldBeNil)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("ListUUIDs with error", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		_, err = client.ListUUIDs(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "fake error")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("GetTransform with error", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		_, err = client.GetTransform(context.Background(), []byte("test-uuid"), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "fake error")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("StreamTransformChanges with error", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := worldstatestore.NewClientFromConn(
			context.Background(),
			conn,
			"",
			worldstatestore.Named(testWorldStateStoreServiceName),
			logger,
		)
		test.That(t, err, test.ShouldBeNil)

		_, err = client.StreamTransformChanges(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "fake error")

		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
}

package worldstatestore_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
	"go.viam.com/rdk/testutils/inject"
)

const testWorldStateStoreServiceName = "worldstatestore1"

func newServer(m map[resource.Name]worldstatestore.Service, logger logging.Logger) (pb.WorldStateStoreServiceServer, error) {
	coll, err := resource.NewAPIResourceCollection(worldstatestore.API, m)
	if err != nil {
		return nil, err
	}
	return worldstatestore.NewRPCServiceServer(coll, logger).(pb.WorldStateStoreServiceServer), nil
}

func TestWorldStateStoreServerFailures(t *testing.T) {
	// Test with no service
	logger := logging.NewTestLogger(t)
	m := map[resource.Name]worldstatestore.Service{}
	server, err := newServer(m, logger)
	test.That(t, err, test.ShouldBeNil)

	// Test ListUUIDs with no service
	_, err = server.ListUUIDs(context.Background(), &pb.ListUUIDsRequest{Name: testWorldStateStoreServiceName})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	// Test GetTransform with no service
	_, err = server.GetTransform(
		context.Background(),
		&pb.GetTransformRequest{Name: testWorldStateStoreServiceName, Uuid: []byte("test-uuid")},
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	// Test StreamTransformChanges with no service
	req := &pb.StreamTransformChangesRequest{Name: testWorldStateStoreServiceName}
	mockStream := &mockStreamTransformChangesServer{ctx: context.Background()}
	err = server.StreamTransformChanges(req, mockStream)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	// Test DoCommand with no service
	_, err = server.DoCommand(context.Background(), &commonpb.DoCommandRequest{Name: testWorldStateStoreServiceName})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
}

func TestServerListUUIDs(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectWSS := &inject.WorldStateStoreService{}
	m := map[resource.Name]worldstatestore.Service{
		worldstatestore.Named(testWorldStateStoreServiceName): injectWSS,
	}
	server, err := newServer(m, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("successful ListUUIDs", func(t *testing.T) {
		expectedUUIDs := [][]byte{[]byte("uuid1"), []byte("uuid2"), []byte("uuid3")}
		extra := map[string]interface{}{"foo": "bar"}
		ext, err := structpb.NewStruct(extra)
		test.That(t, err, test.ShouldBeNil)

		injectWSS.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
			return expectedUUIDs, nil
		}

		req := &pb.ListUUIDsRequest{
			Name:  testWorldStateStoreServiceName,
			Extra: ext,
		}

		resp, err := server.ListUUIDs(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Uuids, test.ShouldResemble, expectedUUIDs)
	})

	t.Run("ListUUIDs with error", func(t *testing.T) {
		expectedErr := errors.New("fake error")
		injectWSS.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
			return nil, expectedErr
		}

		req := &pb.ListUUIDsRequest{Name: testWorldStateStoreServiceName}
		_, err := server.ListUUIDs(context.Background(), req)
		test.That(t, err, test.ShouldEqual, expectedErr)
	})

	t.Run("ListUUIDs with nil response", func(t *testing.T) {
		injectWSS.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
			return nil, nil
		}

		req := &pb.ListUUIDsRequest{Name: testWorldStateStoreServiceName}
		_, err := server.ListUUIDs(context.Background(), req)
		test.That(t, err, test.ShouldEqual, worldstatestore.ErrNilResponse)
	})
}

func TestServerGetTransform(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectWSS := &inject.WorldStateStoreService{}
	m := map[resource.Name]worldstatestore.Service{
		worldstatestore.Named(testWorldStateStoreServiceName): injectWSS,
	}
	server, err := newServer(m, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("successful GetTransform", func(t *testing.T) {
		expectedTransform := &commonpb.Transform{
			ReferenceFrame: "test-frame",
			Uuid:           []byte("test-uuid"),
		}
		extra := map[string]interface{}{"foo": "bar"}
		ext, err := structpb.NewStruct(extra)
		test.That(t, err, test.ShouldBeNil)

		injectWSS.GetTransformFunc = func(
			ctx context.Context,
			uuid []byte,
			extra map[string]any,
		) (*commonpb.Transform, error) {
			return expectedTransform, nil
		}

		req := &pb.GetTransformRequest{
			Name:  testWorldStateStoreServiceName,
			Uuid:  []byte("test-uuid"),
			Extra: ext,
		}

		resp, err := server.GetTransform(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Transform, test.ShouldResemble, expectedTransform)
	})

	t.Run("GetTransform with error", func(t *testing.T) {
		expectedErr := errors.New("fake error")
		injectWSS.GetTransformFunc = func(
			ctx context.Context,
			uuid []byte,
			extra map[string]any,
		) (*commonpb.Transform, error) {
			return nil, expectedErr
		}

		req := &pb.GetTransformRequest{
			Name: testWorldStateStoreServiceName,
			Uuid: []byte("test-uuid"),
		}
		_, err := server.GetTransform(context.Background(), req)
		test.That(t, err, test.ShouldEqual, expectedErr)
	})

	t.Run("GetTransform with nil response", func(t *testing.T) {
		injectWSS.GetTransformFunc = func(
			ctx context.Context,
			uuid []byte,
			extra map[string]any,
		) (*commonpb.Transform, error) {
			return nil, nil
		}

		req := &pb.GetTransformRequest{
			Name: testWorldStateStoreServiceName,
			Uuid: []byte("test-uuid"),
		}
		resp, err := server.GetTransform(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Transform, test.ShouldBeNil)
	})
}

func TestServerStreamTransformChanges(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectWSS := &inject.WorldStateStoreService{}
	m := map[resource.Name]worldstatestore.Service{
		worldstatestore.Named(testWorldStateStoreServiceName): injectWSS,
	}
	server, err := newServer(m, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("successful StreamTransformChanges", func(t *testing.T) {
		extra := map[string]interface{}{"foo": "bar"}
		ext, err := structpb.NewStruct(extra)
		test.That(t, err, test.ShouldBeNil)

		changesChan := make(chan worldstatestore.TransformChange, 2)
		changesChan <- worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform: &commonpb.Transform{
				ReferenceFrame: "test-frame",
				Uuid:           []byte("test-uuid"),
			},
		}
		changesChan <- worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
			Transform: &commonpb.Transform{
				ReferenceFrame: "test-frame",
				Uuid:           []byte("test-uuid"),
			},
			UpdatedFields: []string{"pose_in_observer_frame"},
		}
		close(changesChan)

		injectWSS.StreamTransformChangesFunc = func(
			ctx context.Context,
			extra map[string]any,
		) (*worldstatestore.TransformChangeStream, error) {
			return worldstatestore.NewTransformChangeStreamFromChannel(ctx, changesChan), nil
		}

		req := &pb.StreamTransformChangesRequest{
			Name:  testWorldStateStoreServiceName,
			Extra: ext,
		}

		// Create a mock stream
		mockStream := &mockStreamTransformChangesServer{
			ctx:     context.Background(),
			changes: make([]*pb.StreamTransformChangesResponse, 0),
		}

		err = server.StreamTransformChanges(req, mockStream)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(mockStream.changes), test.ShouldEqual, 3) // 1 empty + 2 changes
		if len(mockStream.changes) == 3 {
			updated := mockStream.changes[2]
			test.That(t, updated.UpdatedFields, test.ShouldNotBeNil)
			test.That(t, updated.UpdatedFields.Paths, test.ShouldResemble, []string{"pose_in_observer_frame"})
		}
	})

	t.Run("StreamTransformChanges with error", func(t *testing.T) {
		expectedErr := errors.New("fake error")
		injectWSS.StreamTransformChangesFunc = func(
			ctx context.Context,
			extra map[string]any,
		) (*worldstatestore.TransformChangeStream, error) {
			return nil, expectedErr
		}

		req := &pb.StreamTransformChangesRequest{Name: testWorldStateStoreServiceName}
		mockStream := &mockStreamTransformChangesServer{
			ctx: context.Background(),
		}

		err := server.StreamTransformChanges(req, mockStream)
		test.That(t, err, test.ShouldEqual, expectedErr)
	})
}

func TestServerDoCommand(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectWSS := &inject.WorldStateStoreService{}
	m := map[resource.Name]worldstatestore.Service{
		worldstatestore.Named(testWorldStateStoreServiceName): injectWSS,
	}
	server, err := newServer(m, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("successful DoCommand", func(t *testing.T) {
		expectedResponse := map[string]interface{}{"result": "success"}
		injectWSS.DoFunc = func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
			return expectedResponse, nil
		}

		req := &commonpb.DoCommandRequest{Name: testWorldStateStoreServiceName}
		resp, err := server.DoCommand(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
	})
}

// mockStreamTransformChangesServer implements pb.WorldStateStoreService_StreamTransformChangesServer for testing.
type mockStreamTransformChangesServer struct {
	grpc.ServerStream
	ctx     context.Context
	changes []*pb.StreamTransformChangesResponse
}

func (m *mockStreamTransformChangesServer) Context() context.Context {
	return m.ctx
}

func (m *mockStreamTransformChangesServer) Send(resp *pb.StreamTransformChangesResponse) error {
	m.changes = append(m.changes, resp)
	return nil
}

package cli

import (
	"context"
	"net"
	"testing"

	datasetpb "go.viam.com/api/app/dataset/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

// startMockDatasetServer spins up an in-process gRPC server hosting the given
// DatasetServiceServer implementation and returns a connected client + a
// shutdown function.
func startMockDatasetServer(t *testing.T, srv datasetpb.DatasetServiceServer) (datasetpb.DatasetServiceClient, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	gs := grpc.NewServer()
	datasetpb.RegisterDatasetServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	test.That(t, err, test.ShouldBeNil)
	client := datasetpb.NewDatasetServiceClient(conn)
	return client, func() {
		_ = conn.Close()
		gs.Stop()
	}
}

// fakeDatasetServer captures the methods we need to mock; embeds the
// UnimplementedDatasetServiceServer so any unhandled RPC returns Unimplemented.
type fakeDatasetServer struct {
	datasetpb.UnimplementedDatasetServiceServer

	listResponse  *datasetpb.ListDatasetsByIDsResponse
	startResponse *datasetpb.StartSequenceDatasetExportResponse
	getResponses  []*datasetpb.GetSequenceDatasetExportResponse
	getCallCount  int
	startCalled   bool
}

func (f *fakeDatasetServer) ListDatasetsByIDs(_ context.Context, _ *datasetpb.ListDatasetsByIDsRequest) (*datasetpb.ListDatasetsByIDsResponse, error) {
	return f.listResponse, nil
}

func (f *fakeDatasetServer) StartSequenceDatasetExport(_ context.Context, _ *datasetpb.StartSequenceDatasetExportRequest) (*datasetpb.StartSequenceDatasetExportResponse, error) {
	f.startCalled = true
	return f.startResponse, nil
}

func (f *fakeDatasetServer) GetSequenceDatasetExport(_ context.Context, _ *datasetpb.GetSequenceDatasetExportRequest) (*datasetpb.GetSequenceDatasetExportResponse, error) {
	resp := f.getResponses[f.getCallCount]
	if f.getCallCount < len(f.getResponses)-1 {
		f.getCallCount++
	}
	return resp, nil
}

func TestDatasetDownload_DispatchesToSequenceFlow(t *testing.T) {
	fake := &fakeDatasetServer{
		listResponse: &datasetpb.ListDatasetsByIDsResponse{
			Datasets: []*datasetpb.Dataset{{
				Id:   "ds-1",
				Type: datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA,
			}},
		},
	}
	client, shutdown := startMockDatasetServer(t, fake)
	defer shutdown()

	c := &viamClient{datasetClient: client}
	dsType, err := c.lookupDatasetType(context.Background(), "ds-1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dsType, test.ShouldEqual, datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA)
}

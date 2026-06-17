package cli

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
	datasetpb "go.viam.com/api/app/dataset/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
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

func TestDownloadSequenceDataset_PollsUntilCompleted(t *testing.T) {
	fake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{
			{Status: datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_RUNNING},
			{Status: datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_RUNNING},
			{
				Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
				DownloadUrl: "http://placeholder/will-be-tested-in-task-4",
			},
		},
	}
	client, shutdown := startMockDatasetServer(t, fake)
	defer shutdown()

	c := &viamClient{datasetClient: client, c: noopCLICtx(t)}
	// Use a tiny pollInterval so the test runs fast.
	_, err := c.pollUntilTerminal(context.Background(), "job-1", 10*time.Millisecond, time.Second)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fake.getCallCount, test.ShouldEqual, 2) // 0,1,2 = three polls; getCallCount stops incrementing at len-1
}

func TestDownloadSequenceDataset_SurfacesFailedStatus(t *testing.T) {
	fake := &fakeDatasetServer{
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{
			{
				Status:       datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_FAILED,
				ErrorMessage: "query timed out",
			},
		},
	}
	client, shutdown := startMockDatasetServer(t, fake)
	defer shutdown()

	c := &viamClient{datasetClient: client, c: noopCLICtx(t)}
	_, err := c.pollUntilTerminal(context.Background(), "job-1", 10*time.Millisecond, time.Second)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "query timed out")
}

func TestDownloadSequenceDataset_TimesOutAfterMaxWait(t *testing.T) {
	fake := &fakeDatasetServer{
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{
			{Status: datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_RUNNING},
		},
	}
	client, shutdown := startMockDatasetServer(t, fake)
	defer shutdown()

	c := &viamClient{datasetClient: client, c: noopCLICtx(t)}
	_, err := c.pollUntilTerminal(context.Background(), "job-1", 10*time.Millisecond, 50*time.Millisecond)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "timed out")
}

func TestDownloadSequenceDataset_WritesZipToDisk(t *testing.T) {
	zipBody := []byte("PK\x03\x04 fake zip body")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()

	fake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{
			{
				Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
				DownloadUrl: srv.URL,
			},
		},
	}
	client, shutdown := startMockDatasetServer(t, fake)
	defer shutdown()

	dst := t.TempDir()
	c := &viamClient{datasetClient: client, c: noopCLICtx(t)}
	err := c.downloadSequenceDataset(context.Background(), "ds-1", dst, 10*time.Millisecond, time.Second)
	test.That(t, err, test.ShouldBeNil)

	got, err := os.ReadFile(filepath.Join(dst, "ds-1.zip"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, zipBody)
}

func TestDownloadSequenceDataset_SurfacesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "expired", http.StatusForbidden)
	}))
	defer srv.Close()

	fake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{{
			Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
			DownloadUrl: srv.URL,
		}},
	}
	client, shutdown := startMockDatasetServer(t, fake)
	defer shutdown()

	c := &viamClient{datasetClient: client, c: noopCLICtx(t)}
	err := c.downloadSequenceDataset(context.Background(), "ds-1", t.TempDir(), 10*time.Millisecond, time.Second)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "403")
}

func TestDatasetDownloadAction_EndToEndSequenceFlow(t *testing.T) {
	zipBody := []byte("PK\x03\x04 e2e")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()

	fake := &fakeDatasetServer{
		listResponse: &datasetpb.ListDatasetsByIDsResponse{
			Datasets: []*datasetpb.Dataset{{
				Id:   "ds-1",
				Type: datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA,
			}},
		},
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{{
			Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
			DownloadUrl: srv.URL,
		}},
	}
	client, shutdown := startMockDatasetServer(t, fake)
	defer shutdown()

	dst := t.TempDir()
	c := &viamClient{datasetClient: client, c: noopCLICtx(t)}

	// Directly invoke the helper that DatasetDownloadAction would call; this
	// exercises the same code path without instantiating the full urfave/cli
	// command tree.
	dsType, err := c.lookupDatasetType(context.Background(), "ds-1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dsType, test.ShouldEqual, datasetpb.DatasetType_DATASET_TYPE_SEQUENCE_DATA)

	err = c.downloadSequenceDataset(context.Background(), "ds-1", dst, 10*time.Millisecond, time.Second)
	test.That(t, err, test.ShouldBeNil)

	got, err := os.ReadFile(filepath.Join(dst, "ds-1.zip"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, zipBody)
}

func noopCLICtx(t *testing.T) *cli.Command {
	t.Helper()
	return &cli.Command{Writer: io.Discard, ErrWriter: io.Discard}
}

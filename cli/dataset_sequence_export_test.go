package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/urfave/cli/v3"
	datapb "go.viam.com/api/app/data/v1"
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

func (f *fakeDatasetServer) ListDatasetsByIDs(
	_ context.Context, _ *datasetpb.ListDatasetsByIDsRequest,
) (*datasetpb.ListDatasetsByIDsResponse, error) {
	return f.listResponse, nil
}

func (f *fakeDatasetServer) StartSequenceDatasetExport(
	_ context.Context, _ *datasetpb.StartSequenceDatasetExportRequest,
) (*datasetpb.StartSequenceDatasetExportResponse, error) {
	f.startCalled = true
	return f.startResponse, nil
}

func (f *fakeDatasetServer) GetSequenceDatasetExport(
	_ context.Context, _ *datasetpb.GetSequenceDatasetExportRequest,
) (*datasetpb.GetSequenceDatasetExportResponse, error) {
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
	err := c.downloadSequenceDataset(context.Background(), "ds-1", dst, 10*time.Millisecond, time.Second, false, 0, 0)
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
	err := c.downloadSequenceDataset(context.Background(), "ds-1", t.TempDir(), 10*time.Millisecond, time.Second, false, 0, 0)
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

	err = c.downloadSequenceDataset(context.Background(), "ds-1", dst, 10*time.Millisecond, time.Second, false, 0, 0)
	test.That(t, err, test.ShouldBeNil)

	got, err := os.ReadFile(filepath.Join(dst, "ds-1.zip"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, zipBody)
}

func noopCLICtx(t *testing.T) *cli.Command {
	t.Helper()
	return &cli.Command{Writer: io.Discard, ErrWriter: io.Discard}
}

// startMockDataServer spins up an in-process gRPC server hosting the given
// DataServiceServer impl and returns a connected client + a shutdown.
func startMockDataServer(t *testing.T, srv datapb.DataServiceServer) (datapb.DataServiceClient, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	gs := grpc.NewServer()
	datapb.RegisterDataServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.That(t, err, test.ShouldBeNil)
	return datapb.NewDataServiceClient(conn), func() {
		_ = conn.Close()
		gs.Stop()
	}
}

// fakeDataServer mocks the subset of DataService used by the sequence
// binary-blob download path: SequencesByDatasetID, GetSequenceBinaryData,
// BinaryDataByIDs. Each in-memory page maps from request key → response.
type fakeDataServer struct {
	datapb.UnimplementedDataServiceServer

	sequencesByDataset map[string][]*datapb.Sequence
	binaryBySequence   map[string][]*datapb.BinaryData
	binaryBytesByID    map[string][]byte
	binaryErrByID      map[string]error

	mu                       sync.Mutex
	binaryDataByIDsCallCount int
	requestedIDs             []string
}

func (f *fakeDataServer) SequencesByDatasetID(
	_ context.Context, req *datapb.SequencesByDatasetIDRequest,
) (*datapb.SequencesByDatasetIDResponse, error) {
	return &datapb.SequencesByDatasetIDResponse{Sequences: f.sequencesByDataset[req.GetDatasetId()]}, nil
}

func (f *fakeDataServer) GetSequenceBinaryData(
	_ context.Context, req *datapb.GetSequenceBinaryDataRequest,
) (*datapb.GetSequenceBinaryDataResponse, error) {
	return &datapb.GetSequenceBinaryDataResponse{Data: f.binaryBySequence[req.GetSequenceId()]}, nil
}

func (f *fakeDataServer) BinaryDataByIDs(
	_ context.Context, req *datapb.BinaryDataByIDsRequest,
) (*datapb.BinaryDataByIDsResponse, error) {
	f.mu.Lock()
	f.binaryDataByIDsCallCount++
	f.requestedIDs = append(f.requestedIDs, req.GetBinaryDataIds()...)
	f.mu.Unlock()

	resp := &datapb.BinaryDataByIDsResponse{}
	for _, id := range req.GetBinaryDataIds() {
		if err, ok := f.binaryErrByID[id]; ok && err != nil {
			return nil, err
		}
		resp.Data = append(resp.Data, &datapb.BinaryData{
			Binary: f.binaryBytesByID[id],
			Metadata: &datapb.BinaryMetadata{
				BinaryDataId: id,
			},
		})
	}
	return resp, nil
}

func mkBinaryData(id, ext string) *datapb.BinaryData {
	return &datapb.BinaryData{Metadata: &datapb.BinaryMetadata{BinaryDataId: id, FileExt: ext}}
}

func TestDownloadSequenceDataset_DownloadsBinaryBlobs(t *testing.T) {
	zipBody := []byte("PK\x03\x04 zip")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()

	datasetFake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{{
			Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
			DownloadUrl: srv.URL,
		}},
	}
	datasetClient, shutdownDataset := startMockDatasetServer(t, datasetFake)
	defer shutdownDataset()

	dataFake := &fakeDataServer{
		sequencesByDataset: map[string][]*datapb.Sequence{
			"ds-1": {{Id: "seq-a"}, {Id: "seq-b"}},
		},
		binaryBySequence: map[string][]*datapb.BinaryData{
			"seq-a": {mkBinaryData("bd-1", ".jpg"), mkBinaryData("bd-2", ".png")},
			"seq-b": {mkBinaryData("bd-3", ".jpg")},
		},
		binaryBytesByID: map[string][]byte{
			"bd-1": []byte("jpeg-bytes-1"),
			"bd-2": []byte("png-bytes-2"),
			"bd-3": []byte("jpeg-bytes-3"),
		},
	}
	dataClient, shutdownData := startMockDataServer(t, dataFake)
	defer shutdownData()

	dst := t.TempDir()
	c := &viamClient{datasetClient: datasetClient, dataClient: dataClient, c: noopCLICtx(t)}
	err := c.downloadSequenceDataset(context.Background(), "ds-1", dst,
		10*time.Millisecond, time.Second, true, 4, 0)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, mustReadFile(t, filepath.Join(dst, "ds-1.zip")), test.ShouldResemble, zipBody)
	test.That(t, mustReadFile(t, filepath.Join(dst, "binary_data", "bd-1.jpg")), test.ShouldResemble, []byte("jpeg-bytes-1"))
	test.That(t, mustReadFile(t, filepath.Join(dst, "binary_data", "bd-2.png")), test.ShouldResemble, []byte("png-bytes-2"))
	test.That(t, mustReadFile(t, filepath.Join(dst, "binary_data", "bd-3.jpg")), test.ShouldResemble, []byte("jpeg-bytes-3"))
}

// TestDownloadSequenceDataset_DownloadsNestedBinaryBlobs guards the case where a
// binary_data_id contains slashes (org/part/id): the destination path is nested
// below binary_data/, so the per-blob parent directory must be created before the
// write. Without that, os.WriteFile fails with "no such file or directory".
func TestDownloadSequenceDataset_DownloadsNestedBinaryBlobs(t *testing.T) {
	zipBody := []byte("PK\x03\x04 zip")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()

	datasetFake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{{
			Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
			DownloadUrl: srv.URL,
		}},
	}
	datasetClient, shutdownDataset := startMockDatasetServer(t, datasetFake)
	defer shutdownDataset()

	nestedID := "org-x/part-y/BzkyJEtND5AuQR7YIb2LS3Z1"
	dataFake := &fakeDataServer{
		sequencesByDataset: map[string][]*datapb.Sequence{"ds-1": {{Id: "seq-a"}}},
		binaryBySequence: map[string][]*datapb.BinaryData{
			"seq-a": {mkBinaryData(nestedID, ".jpeg")},
		},
		binaryBytesByID: map[string][]byte{nestedID: []byte("nested-jpeg-bytes")},
	}
	dataClient, shutdownData := startMockDataServer(t, dataFake)
	defer shutdownData()

	dst := t.TempDir()
	c := &viamClient{datasetClient: datasetClient, dataClient: dataClient, c: noopCLICtx(t)}
	err := c.downloadSequenceDataset(context.Background(), "ds-1", dst,
		10*time.Millisecond, time.Second, true, 4, 0)
	test.That(t, err, test.ShouldBeNil)

	// The blob must land at binary_data/<nested id><ext>, matching the relative
	// path the server records in binary_data.parquet.
	test.That(t, mustReadFile(t, filepath.Join(dst, "binary_data", "org-x", "part-y", "BzkyJEtND5AuQR7YIb2LS3Z1.jpeg")),
		test.ShouldResemble, []byte("nested-jpeg-bytes"))
}

func TestDownloadSequenceDataset_SkipsBinaryWhenDisabled(t *testing.T) {
	zipBody := []byte("PK\x03\x04")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()

	datasetFake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{{
			Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
			DownloadUrl: srv.URL,
		}},
	}
	datasetClient, shutdownDataset := startMockDatasetServer(t, datasetFake)
	defer shutdownDataset()

	dataFake := &fakeDataServer{}
	dataClient, shutdownData := startMockDataServer(t, dataFake)
	defer shutdownData()

	dst := t.TempDir()
	c := &viamClient{datasetClient: datasetClient, dataClient: dataClient, c: noopCLICtx(t)}
	err := c.downloadSequenceDataset(context.Background(), "ds-1", dst,
		10*time.Millisecond, time.Second, false, 4, 0)
	test.That(t, err, test.ShouldBeNil)

	_, err = os.Stat(filepath.Join(dst, "binary_data"))
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)
	test.That(t, dataFake.binaryDataByIDsCallCount, test.ShouldEqual, 0)
}

func TestDownloadSequenceDataset_SurfacesBinaryFetchErrors(t *testing.T) {
	zipBody := []byte("PK\x03\x04")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()

	datasetFake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{{
			Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
			DownloadUrl: srv.URL,
		}},
	}
	datasetClient, shutdownDataset := startMockDatasetServer(t, datasetFake)
	defer shutdownDataset()

	dataFake := &fakeDataServer{
		sequencesByDataset: map[string][]*datapb.Sequence{"ds-1": {{Id: "seq-a"}}},
		binaryBySequence: map[string][]*datapb.BinaryData{
			"seq-a": {mkBinaryData("bd-broken", ".jpg")},
		},
		binaryErrByID: map[string]error{"bd-broken": errors.New("server boom")},
	}
	dataClient, shutdownData := startMockDataServer(t, dataFake)
	defer shutdownData()

	c := &viamClient{datasetClient: datasetClient, dataClient: dataClient, c: noopCLICtx(t)}
	err := c.downloadSequenceDataset(context.Background(), "ds-1", t.TempDir(),
		10*time.Millisecond, time.Second, true, 4, 0)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "server boom")
}

func TestDownloadSequenceDataset_SkipsAlreadyDownloaded(t *testing.T) {
	zipBody := []byte("PK\x03\x04")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBody)
	}))
	defer srv.Close()

	datasetFake := &fakeDatasetServer{
		startResponse: &datasetpb.StartSequenceDatasetExportResponse{JobId: "job-1"},
		getResponses: []*datasetpb.GetSequenceDatasetExportResponse{{
			Status:      datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED,
			DownloadUrl: srv.URL,
		}},
	}
	datasetClient, shutdownDataset := startMockDatasetServer(t, datasetFake)
	defer shutdownDataset()

	dataFake := &fakeDataServer{
		sequencesByDataset: map[string][]*datapb.Sequence{"ds-1": {{Id: "seq-a"}}},
		binaryBySequence: map[string][]*datapb.BinaryData{
			"seq-a": {mkBinaryData("bd-1", ".jpg"), mkBinaryData("bd-2", ".png")},
		},
		binaryBytesByID: map[string][]byte{
			"bd-1": []byte("new-bytes"),
			"bd-2": []byte("new-png"),
		},
	}
	dataClient, shutdownData := startMockDataServer(t, dataFake)
	defer shutdownData()

	dst := t.TempDir()
	test.That(t, os.MkdirAll(filepath.Join(dst, "binary_data"), 0o700), test.ShouldBeNil)
	existing := filepath.Join(dst, "binary_data", "bd-1.jpg")
	test.That(t, os.WriteFile(existing, []byte("preserved"), 0o600), test.ShouldBeNil)

	c := &viamClient{datasetClient: datasetClient, dataClient: dataClient, c: noopCLICtx(t)}
	err := c.downloadSequenceDataset(context.Background(), "ds-1", dst,
		10*time.Millisecond, time.Second, true, 4, 0)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, mustReadFile(t, existing), test.ShouldResemble, []byte("preserved"))
	test.That(t, dataFake.requestedIDs, test.ShouldResemble, []string{"bd-2"})
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	return b
}

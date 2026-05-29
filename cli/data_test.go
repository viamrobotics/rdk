package cli

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestFilenameForDownload(t *testing.T) {
	const expectedUTC = "1970-01-01T00_00_00Z"
	noFilename := filenameForDownload(&datapb.BinaryMetadata{Id: "my-id"})
	test.That(t, noFilename, test.ShouldEqual, expectedUTC+"_my-id")

	normalExt := filenameForDownload(&datapb.BinaryMetadata{FileName: "whatever.txt"})
	test.That(t, normalExt, test.ShouldEqual, expectedUTC+"_whatever.txt")

	inFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "dir/whatever.txt"})
	test.That(t, inFolder, test.ShouldEqual, "dir/whatever.txt")

	inViamCaptureFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "/.viam/capture/2024-01-30Twhatever.jpg"})
	test.That(t, inViamCaptureFolder, test.ShouldEqual, "2024-01-30Twhatever.jpg")

	nestedViamCaptureFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "Users/hi/.viam/capture/2024-01-30Twhatever.jpg"})
	test.That(t, nestedViamCaptureFolder, test.ShouldEqual, "2024-01-30Twhatever.jpg")

	nestedDirViamCaptureFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "Users/hi/.viam/capture/dir/2024-01-30Twhatever.jpg"})
	test.That(t, nestedDirViamCaptureFolder, test.ShouldEqual, "dir/2024-01-30Twhatever.jpg")

	gzAtRoot := filenameForDownload(&datapb.BinaryMetadata{FileName: "whatever.gz"})
	test.That(t, gzAtRoot, test.ShouldEqual, expectedUTC+"_whatever")

	gzInFolder := filenameForDownload(&datapb.BinaryMetadata{FileName: "dir/whatever.gz"})
	test.That(t, gzInFolder, test.ShouldEqual, "dir/whatever")
}

func TestDownloadBinarySkipsExisting(t *testing.T) {
	dst := t.TempDir()

	// Two ids: "have" already exists on disk non-zero; "missing" does not.
	fileNames := map[string]string{"have": "have.jpg", "missing": "missing.jpg"}
	// newMeta returns a fresh metadata object per call, matching real gRPC
	// unmarshaling (downloadBinary mutates metadata.FileName in place).
	newMeta := func(id string) *datapb.BinaryMetadata {
		return &datapb.BinaryMetadata{Id: id, FileName: fileNames[id], FileExt: ".jpg"}
	}

	// Pre-create the "have" data file with non-zero contents at the path the
	// downloader would use.
	havePath := dataFilePath(dst, filenameForDownload(newMeta("have")), ".jpg")
	test.That(t, os.MkdirAll(filepath.Dir(havePath), 0o700), test.ShouldBeNil)
	test.That(t, os.WriteFile(havePath, []byte("existing"), 0o600), test.ShouldBeNil)

	var binaryRequestedIDs atomic.Value
	dsc := &inject.DataServiceClient{
		BinaryDataByIDsFunc: func(_ context.Context, in *datapb.BinaryDataByIDsRequest, _ ...grpc.CallOption,
		) (*datapb.BinaryDataByIDsResponse, error) {
			resp := &datapb.BinaryDataByIDsResponse{}
			for _, id := range in.GetBinaryDataIds() {
				datum := &datapb.BinaryData{Metadata: newMeta(id)}
				if in.GetIncludeBinary() {
					datum.Binary = []byte("fresh-bytes")
				}
				resp.Data = append(resp.Data, datum)
			}
			if in.GetIncludeBinary() {
				binaryRequestedIDs.Store(append([]string{}, in.GetBinaryDataIds()...))
			}
			return resp, nil
		},
	}

	cCtx, ac, _, _ := setup(&inject.AppServiceClient{}, dsc, nil, nil, "token")
	_ = cCtx

	err := ac.downloadBinary(context.Background(), dst, 0, "have", "missing")
	test.That(t, err, test.ShouldBeNil)

	// Only "missing" should have triggered a binary (IncludeBinary) request.
	requested, _ := binaryRequestedIDs.Load().([]string)
	test.That(t, requested, test.ShouldResemble, []string{"missing"})

	// The pre-existing file must be untouched (not overwritten with fresh bytes).
	haveContents, err := os.ReadFile(havePath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(haveContents), test.ShouldEqual, "existing")

	// The missing file should have been downloaded.
	missingPath := dataFilePath(dst, filenameForDownload(newMeta("missing")), ".jpg")
	missingContents, err := os.ReadFile(missingPath)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(missingContents), test.ShouldEqual, "fresh-bytes")
}

package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/benbjohnson/clock"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
)

// newTestSync builds a Sync wired to a mock cloud client. If connected is true the
// cloud connection is marked ready. It is torn down via t.Cleanup.
func newTestSync(t *testing.T, client v1.DataSyncServiceClient, connected bool) *Sync {
	t.Helper()
	s := New(
		func(grpc.ClientConnInterface) v1.DataSyncServiceClient { return client },
		func() {}, // flushCollectors no-op
		clock.New(),
		logging.NewTestLogger(t),
	)
	s.cloudConn.partID = "my-part-id"
	s.cloudConn.client = client
	if connected {
		close(s.cloudConn.ready)
	}
	t.Cleanup(s.Close)
	return s
}

func writeTestFile(t *testing.T, dir, name string, contents []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	test.That(t, os.WriteFile(p, contents, 0o600), test.ShouldBeNil)
	return p
}

func fileUploadClientReturningIDs(t *testing.T, ids ...string) MockDataSyncServiceClient {
	t.Helper()
	var i int
	return MockDataSyncServiceClient{
		T: t,
		FileUploadFunc: func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error) {
			id := ids[i]
			i++
			return &ClientStreamingMock[*v1.FileUploadRequest, *v1.FileUploadResponse]{
				T:                t,
				SendFunc:         func(*v1.FileUploadRequest) error { return nil },
				CloseAndRecvFunc: func() (*v1.FileUploadResponse, error) { return &v1.FileUploadResponse{BinaryDataId: id}, nil },
			}, nil
		},
	}
}

func TestUploadDataFromPath(t *testing.T) {
	ctx := context.Background()

	t.Run("not connected to the cloud", func(t *testing.T) {
		dir := t.TempDir()
		s := newTestSync(t, NoOpCloudClientConstructor(nil), false)
		_, err := s.UploadDataFromPath(ctx, writeTestFile(t, dir, "f.txt", []byte("hi")), nil, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not connected to the cloud")
	})

	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		s := newTestSync(t, fileUploadClientReturningIDs(t, "bin-1"), true)
		contents := []byte("hello world")
		fp := writeTestFile(t, dir, "f.txt", contents)

		res, err := s.UploadDataFromPath(ctx, fp, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res.FilesUploaded, test.ShouldEqual, uint64(1))
		test.That(t, res.FilesFailed, test.ShouldEqual, uint64(0))
		test.That(t, res.BytesUploaded, test.ShouldEqual, uint64(len(contents)))
		test.That(t, res.BytesTotal, test.ShouldEqual, uint64(len(contents)))
		test.That(t, res.IDs, test.ShouldResemble, []string{"bin-1"})

		// arbitrary files are deleted after a successful upload
		_, statErr := os.Stat(fp)
		test.That(t, os.IsNotExist(statErr), test.ShouldBeTrue)
	})

	t.Run("directory of files", func(t *testing.T) {
		dir := t.TempDir()
		s := newTestSync(t, fileUploadClientReturningIDs(t, "bin-1", "bin-2", "bin-3"), true)
		uploadDir := filepath.Join(dir, "uploads")
		test.That(t, os.MkdirAll(uploadDir, 0o700), test.ShouldBeNil)
		a, b, c := []byte("aaa"), []byte("bbbb"), []byte("ccccc")
		writeTestFile(t, uploadDir, "a.txt", a)
		writeTestFile(t, uploadDir, "b.txt", b)
		writeTestFile(t, uploadDir, "c.txt", c)

		res, err := s.UploadDataFromPath(ctx, uploadDir, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res.FilesUploaded, test.ShouldEqual, uint64(3))
		test.That(t, res.FilesFailed, test.ShouldEqual, uint64(0))
		total := uint64(len(a) + len(b) + len(c))
		test.That(t, res.BytesUploaded, test.ShouldEqual, total)
		test.That(t, res.BytesTotal, test.ShouldEqual, total)
		test.That(t, res.IDs, test.ShouldResemble, []string{"bin-1", "bin-2", "bin-3"})
	})

	t.Run("directory with a partial failure", func(t *testing.T) {
		dir := t.TempDir()
		s := newTestSync(t, NoOpCloudClientConstructor(nil), true)
		uploadDir := filepath.Join(dir, "uploads")
		test.That(t, os.MkdirAll(uploadDir, 0o700), test.ShouldBeNil)

		good := []byte("good")
		writeTestFile(t, uploadDir, "good.txt", good)

		// broken symlink fails os.Stat for every user
		badPath := filepath.Join(uploadDir, "bad.txt")
		test.That(t, os.Symlink(filepath.Join(uploadDir, "no-such-target"), badPath), test.ShouldBeNil)

		res, err := s.UploadDataFromPath(ctx, uploadDir, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res.FilesUploaded, test.ShouldEqual, uint64(1))
		test.That(t, res.FilesFailed, test.ShouldEqual, uint64(1))
		test.That(t, res.BytesUploaded, test.ShouldEqual, uint64(len(good))) // only the good file uploaded
		test.That(t, res.BytesTotal, test.ShouldEqual, uint64(len(good)))    // broken symlink failed stat, so not included in total
	})
}

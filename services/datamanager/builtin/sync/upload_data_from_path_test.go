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
func newTestSync(t *testing.T, client v1.DataSyncServiceClient, captureDir string, connected bool) *Sync {
	t.Helper()
	s := New(
		func(grpc.ClientConnInterface) v1.DataSyncServiceClient { return client },
		func() {}, // flushCollectors no-op
		clock.New(),
		logging.NewTestLogger(t),
	)
	s.config = Config{CaptureDir: captureDir}
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

func fileUploadClientReturningID(t *testing.T, id string) MockDataSyncServiceClient {
	t.Helper()
	return MockDataSyncServiceClient{
		T: t,
		FileUploadFunc: func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error) {
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
		s := newTestSync(t, NoOpCloudClientConstructor(nil), dir, false)
		_, _, _, _, _, err := s.UploadDataFromPath(ctx, writeTestFile(t, dir, "f.txt", []byte("hi")), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not connected to the cloud")
	})

	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		s := newTestSync(t, fileUploadClientReturningID(t, "bin-1"), dir, true)
		contents := []byte("hello world")
		fp := writeTestFile(t, dir, "f.txt", contents)

		fu, ff, bu, bt, ids, err := s.UploadDataFromPath(ctx, fp, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fu, test.ShouldEqual, uint64(1))
		test.That(t, ff, test.ShouldEqual, uint64(0))
		test.That(t, bu, test.ShouldEqual, uint64(len(contents)))
		test.That(t, bt, test.ShouldEqual, uint64(len(contents)))
		test.That(t, ids, test.ShouldResemble, []string{"bin-1"})

		// arbitrary files are deleted after a successful upload
		_, statErr := os.Stat(fp)
		test.That(t, os.IsNotExist(statErr), test.ShouldBeTrue)
	})

	t.Run("directory of files", func(t *testing.T) {
		dir := t.TempDir()
		s := newTestSync(t, fileUploadClientReturningID(t, "bin-1"), dir, true)
		uploadDir := filepath.Join(dir, "uploads")
		test.That(t, os.MkdirAll(uploadDir, 0o700), test.ShouldBeNil)
		a, b, c := []byte("aaa"), []byte("bbbb"), []byte("ccccc")
		writeTestFile(t, uploadDir, "a.txt", a)
		writeTestFile(t, uploadDir, "b.txt", b)
		writeTestFile(t, uploadDir, "c.txt", c)

		fu, ff, bu, bt, ids, err := s.UploadDataFromPath(ctx, uploadDir, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fu, test.ShouldEqual, uint64(3))
		test.That(t, ff, test.ShouldEqual, uint64(0))
		total := uint64(len(a) + len(b) + len(c))
		test.That(t, bu, test.ShouldEqual, total)
		test.That(t, bt, test.ShouldEqual, total)
		test.That(t, len(ids), test.ShouldEqual, 3)
	})

	t.Run("directory with a partial failure", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("chmod-based unreadable file does not fail for root")
		}
		dir := t.TempDir()
		s := newTestSync(t, NoOpCloudClientConstructor(nil), dir, true)
		uploadDir := filepath.Join(dir, "uploads")
		test.That(t, os.MkdirAll(uploadDir, 0o700), test.ShouldBeNil)

		good := []byte("good")
		bad := []byte("bad!")
		writeTestFile(t, uploadDir, "good.txt", good)
		badPath := writeTestFile(t, uploadDir, "bad.txt", bad)
		// make bad.txt unreadable so os.Open fails (os.Stat still succeeds)
		test.That(t, os.Chmod(badPath, 0o000), test.ShouldBeNil)
		t.Cleanup(func() { os.Chmod(badPath, 0o600) })

		fu, ff, bu, bt, _, err := s.UploadDataFromPath(ctx, uploadDir, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fu, test.ShouldEqual, uint64(1))
		test.That(t, ff, test.ShouldEqual, uint64(1))
		test.That(t, bu, test.ShouldEqual, uint64(len(good)))          // only the good file uploaded
		test.That(t, bt, test.ShouldEqual, uint64(len(good)+len(bad))) // both discovered
	})
}

package datamanager_test

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func newTestDataManager(t *testing.T, captureDir string) internal.DMService {
	t.Helper()
	dmCfg := &datamanager.Config{
		CaptureDir: captureDir,
	}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: dmCfg,
	}

	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	const arm1Key = "arm1"
	arm1 := &inject.Arm{}
	// Set a dummy GetEndPositionFunc so inject doesn't throw error
	arm1.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return &commonpb.Pose{X: 1, Y: 2, Z: 3}, nil
	}
	rs := map[resource.Name]interface{}{arm.Named(arm1Key): arm1}
	r.MockResourcesFromMap(rs)
	svc, err := datamanager.New(context.Background(), r, cfgService, logger)
	if err != nil {
		t.Log(err)
	}
	return svc.(internal.DMService)
}

func setupConfig(t *testing.T, relativePath string) *config.Config {
	t.Helper()
	logger := golog.NewTestLogger(t)
	testCfg, err := config.Read(context.Background(), rutils.ResolveFile(relativePath), logger)
	test.That(t, err, test.ShouldBeNil)
	return testCfg
}

// readDir filters out folders from a slice of FileInfos.
func readDir(t *testing.T, dir string) ([]fs.FileInfo, error) {
	t.Helper()
	filesAndFolders, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Log(err)
	}
	var onlyFiles []fs.FileInfo
	for _, s := range filesAndFolders {
		if !s.IsDir() {
			onlyFiles = append(onlyFiles, s)
		}
	}
	return onlyFiles, err
}

func resetFolder(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Log(err)
	}
}

// Validates that manual syncing works for a datamanager.
func TestManualSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	configPath := "robots/configs/datamanager_fake.json"
	testCfg := setupConfig(t, configPath)
	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture/arm/arm1/"
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, queueDir)
	defer resetFolder(t, captureDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, captureDir)
	defer dmsvc.Close(context.Background())
	dmsvc.Update(context.Background(), testCfg)
	dmsvc.SetUploadFn(uploadFn)

	// Look at the files in captureDir.
	filesInCaptureDir, err := readDir(t, captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	firstFileInCaptureDir := filesInCaptureDir[0].Name()
	// Give it a second to run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Second)

	filesInCaptureDir, err = readDir(t, captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueueDir, err := readDir(t, queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}

	// We should have uploaded the first file and should now be collecting another one.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	secondFileInCaptureDir := filesInCaptureDir[0].Name()
	test.That(t, firstFileInCaptureDir, test.ShouldNotEqual, secondFileInCaptureDir)
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 1)
}

// Validates that scheduled syncing works for a datamanager.
func TestScheduledSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	configPath := "robots/configs/datamanager_fake.json"
	testCfg := setupConfig(t, configPath)
	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture/arm/arm1/"
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, queueDir)
	defer resetFolder(t, captureDir)

	// Initialize the data manager, update it with our config, and tell it to close later.
	dmsvc := newTestDataManager(t, captureDir)
	defer dmsvc.Close(cancelCtx)
	defer cancelFn()
	dmsvc.Update(cancelCtx, testCfg)
	dmsvc.SetUploadFn(uploadFn)

	// Look at the files in captureDir.
	filesInCaptureDir, err := readDir(t, captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	firstFileInCaptureDir := filesInCaptureDir[0].Name()

	// Queue files and wait for operation to finish, then call cancelFn so we don't queue anymore.
	dmsvc.QueueCapturedData(cancelCtx, 2000)
	time.Sleep(time.Millisecond * 2200)

	// Verify files were enqueued.
	filesInQueueDir, err := readDir(t, queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	// Should have 1 file in the queue since we just moved it there.
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 1)

	filesInCaptureDir, err = readDir(t, captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// We should have uploaded the first file and should now be collecting another one.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	secondFileInCaptureDir := filesInCaptureDir[0].Name()
	test.That(t, firstFileInCaptureDir, test.ShouldNotEqual, secondFileInCaptureDir)

	// Wait a bit for the upload goroutine to trigger on the syncer, then ensure the file was uploaded.
	dmsvc.StartSyncer()
	time.Sleep(time.Millisecond * 600)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 1)
}

// Validates that we can attempt a scheduled and manual sync at the same time without duplicating files or running into errors.
func TestManualAndScheduledSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	configPath := "robots/configs/datamanager_fake.json"
	testCfg := setupConfig(t, configPath)
	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture/arm/arm1/"
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, queueDir)
	defer resetFolder(t, captureDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, captureDir)

	// Make sure we close resources to prevent leaks.
	defer dmsvc.Close(cancelCtx)
	defer cancelFn()
	dmsvc.Update(cancelCtx, testCfg)
	dmsvc.SetUploadFn(uploadFn)

	// Look at the files in captureDir.
	filesInCaptureDir, err := readDir(t, captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)

	// Queue files and perform a manual sync at approximately the same time.
	dmsvc.QueueCapturedData(cancelCtx, 2000)
	time.Sleep(time.Millisecond * 2000)
	dmsvc.Sync(cancelCtx)

	time.Sleep(time.Millisecond * 200)

	// Verify two different files were enqueued.
	filesInQueueDir, err := readDir(t, queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 2)
	test.That(t, filesInQueueDir[0].Name(), test.ShouldNotEqual, filesInQueueDir[1].Name())

	filesInCaptureDir, err = readDir(t, captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// We've queued the first two files and should now be collecting another one.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)

	// Wait for the files to be uploaded, then verify that the correct amount was uploaded.
	time.Sleep(time.Millisecond * 500)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)

	// Verify there are no more
	filesInQueueDir, err = readDir(t, queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 0)
}

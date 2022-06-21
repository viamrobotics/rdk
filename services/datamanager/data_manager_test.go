package datamanager_test

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

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

func newTestDataManager(t *testing.T) internal.DMService {
	t.Helper()
	dmCfg := &datamanager.Config{}
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

func TestNewDataManager(t *testing.T) {
	// Empty config at initialization.
	captureDir := "/tmp/capture"
	svc := newTestDataManager(t)
	// Set capture parameters in Update.
	conf := setupConfig(t, "robots/configs/fake_robot_with_data_manager.json")
	svcConfig, ok, err := datamanager.GetServiceConfig(conf)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, err, test.ShouldBeNil)
	defer resetFolder(t, captureDir)
	svc.Update(context.Background(), conf)
	sleepTime := time.Millisecond * 5
	time.Sleep(sleepTime)

	// Check that the expected collector is running.
	test.That(t, svc.NumCollectors(), test.ShouldEqual, 1)
	expectedMetadata := data.MethodMetadata{Subtype: resource.SubtypeName("arm"), MethodName: "GetEndPosition"}
	present := svc.HasInCollector("arm1", expectedMetadata)
	test.That(t, present, test.ShouldBeTrue)

	// Check that collector is closed.
	err = svc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(sleepTime)
	test.That(t, svc.NumCollectors(), test.ShouldEqual, 0)

	// Check that the collector wrote to a single file.
	files, err := ioutil.ReadDir(svcConfig.CaptureDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 1)
	resetFolder(t, datamanager.SyncQueuePath)
}

// Validates that manual syncing works for a datamanager.
func TestManualSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	configPath := "robots/configs/fake_data_manager.json"
	testCfg := setupConfig(t, configPath)

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/"

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t)
	defer dmsvc.Close(context.Background())
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(context.Background(), testCfg)

	// Look at the files in captureDir.
	filesInArmDir, err := readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
	firstFileInArmDir := filesInArmDir[0].Name()

	// Give it a second to run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Second)

	filesInArmDir, err = readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}

	// We should have uploaded the first file and should now be collecting another one.
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
	secondFileInArmDir := filesInArmDir[0].Name()
	test.That(t, firstFileInArmDir, test.ShouldNotEqual, secondFileInArmDir)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 1)
}

// Validates that scheduled syncing works for a datamanager.
func TestScheduledSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	configPath := "robots/configs/fake_data_manager.json"
	testCfg := setupConfig(t, configPath)

	// Set the sync interval mins to 510ms for the scheduled sync.
	svcConfig, ok, err := datamanager.GetServiceConfig(testCfg)
	if !ok {
		t.Error("malformed/missing datamanager service in config")
	}
	if err != nil {
		t.Error(err)
	}
	svcConfig.SyncIntervalMins = 0.0085

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/"
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, armDir)

	// Initialize the data manager, update it with our config, and tell it to close later.
	dmsvc := newTestDataManager(t)
	defer dmsvc.Close(cancelCtx)
	defer cancelFn()
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(cancelCtx, testCfg)

	// Look at the files in captureDir.
	filesInArmDir, err := readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
	firstFileInArmDir := filesInArmDir[0].Name()

	// We set sync_interval_mins to be about 510ms in the config, so wait 700ms for queueing to occur.
	time.Sleep(time.Millisecond * 700)

	// Wait a bit for the upload goroutine to trigger on the syncer, then ensure the file was uploaded.
	time.Sleep(time.Millisecond * 450)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 1)

	// We should have uploaded the first file and should now be collecting another one.
	filesInArmDir, err = readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
	secondFileInArmDir := filesInArmDir[0].Name()
	test.That(t, firstFileInArmDir, test.ShouldNotEqual, secondFileInArmDir)
}

// Validates that we can attempt a scheduled and manual sync at the same time without duplicating files or running into errors.
func TestManualAndScheduledSync(t *testing.T) {
	var uploadCount uint64
	var uploadedFiles []string
	var lock sync.Mutex
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error {
		lock.Lock()
		atomic.AddUint64(&uploadCount, 1)
		uploadedFiles = append(uploadedFiles, path)
		_ = os.Remove(path)
		lock.Unlock()
		return nil
	}
	configPath := "robots/configs/fake_data_manager.json"
	testCfg := setupConfig(t, configPath)

	// Set the sync interval mins to 510ms for the scheduled sync.
	svcConfig, ok, err := datamanager.GetServiceConfig(testCfg)
	if !ok {
		t.Error("malformed/missing datamanager service in config")
	}
	if err != nil {
		t.Error(err)
	}
	svcConfig.SyncIntervalMins = 0.0085

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1"
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t)

	// Make sure we close resources to prevent leaks.
	defer dmsvc.Close(cancelCtx)
	defer cancelFn()
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(cancelCtx, testCfg)

	// Look at the files in captureDir.
	filesInArmDir, err := readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)

	// Perform a manual and scheduled sync at approximately the same time, then wait for the upload routine to fire.
	time.Sleep(time.Millisecond * 500)
	dmsvc.Sync(cancelCtx)
	time.Sleep(time.Millisecond * 300)

	// Verify two files were uploaded, and that they're different.
	lock.Lock()
	test.That(t, len(uploadedFiles), test.ShouldEqual, 2)
	test.That(t, uploadedFiles[0], test.ShouldNotEqual, uploadedFiles[1])
	lock.Unlock()

	// We've uploaded the first two files and should now be collecting a single new one.
	filesInArmDir, err = readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
}

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

// Get the config associated with the data manager service.
func getServiceConfig(cfg *config.Config) (*config.Service, bool) {
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.ResourceName() == datamanager.Name {
			return &c, true
		}
	}
	return &config.Service{}, false
}

// Validates that manual syncing works for a datamanager.
func TestManualSync(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	configPath := "robots/configs/fake_data_manager.json"
	testCfg := setupConfig(t, configPath)

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/"
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, queueDir)
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, captureDir)
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
	filesInQueueDir, err := readDir(t, queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}

	// We should have uploaded the first file and should now be collecting another one.
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
	secondFileInArmDir := filesInArmDir[0].Name()
	test.That(t, firstFileInArmDir, test.ShouldNotEqual, secondFileInArmDir)
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
	configPath := "robots/configs/fake_data_manager.json"
	testCfg := setupConfig(t, configPath)

	// Set the sync interval mins to 0.0085 for the scheduled sync.
	unconvertedSvcConfig, _ := getServiceConfig(testCfg)
	svcConfig, _ := unconvertedSvcConfig.ConvertedAttributes.(*datamanager.Config)
	svcConfig.SyncIntervalMins = 0.0085

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/"
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, queueDir)
	defer resetFolder(t, armDir)

	// Initialize the data manager, update it with our config, and tell it to close later.
	dmsvc := newTestDataManager(t, captureDir)
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

	// We set sync_interval_mins to be about .5 seconds in the config, so wait 2 seconds for queueing to occur.
	time.Sleep(time.Millisecond * 700)

	// Verify files were enqueued.
	filesInQueueDir, err := readDir(t, queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}

	// Should have 1 file in the queue since we just moved it there.
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 1)

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
	uploadFn := func(ctx context.Context, path string) error {
		lock.Lock()
		atomic.AddUint64(&uploadCount, 1)
		uploadedFiles = append(uploadedFiles, path)
		_ = os.Remove(path)
		lock.Unlock()
		return nil
	}
	configPath := "robots/configs/fake_data_manager.json"
	testCfg := setupConfig(t, configPath)

	// Set the sync interval mins to 0.0085 for the scheduled sync.
	unconvertedSvcConfig, _ := getServiceConfig(testCfg)
	svcConfig, _ := unconvertedSvcConfig.ConvertedAttributes.(*datamanager.Config)
	svcConfig.SyncIntervalMins = 0.0085

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1"
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	// Clear the capture and queue dirs after we're done.
	defer resetFolder(t, queueDir)
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, captureDir)

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
	time.Sleep(time.Millisecond * 500)

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

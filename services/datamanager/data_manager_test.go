package datamanager_test

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	//"github.com/pkg/errors"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"

	//"go.viam.com/rdk/resource"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/test"
)

func getDataManagerServiceConfig(cfg *config.Config) (config.Service, bool) {
	for _, c := range cfg.Services {
		// Compare service type and name.
		if c.ResourceName() == datamanager.Name {
			return c, true
		}
	}
	return config.Service{}, false
}

func newTestDataManager(t *testing.T, testCfg *config.Config, captureDir string) internal.Service {
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
	arm1.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) { // give some dummy GetEndPositionFunc so inject doesn't throw error
		return &commonpb.Pose{X: 1, Y: 2, Z: 3}, nil
	}
	rs := map[resource.Name]interface{}{arm.Named(arm1Key): arm1}
	r.MockResourcesFromMap(rs)
	svc, _ := datamanager.New(context.Background(), r, cfgService, logger)
	return svc.(internal.Service)
}

func setupConfig(t *testing.T, relativePath string) *config.Config {
	logger := golog.NewTestLogger(t)
	testCfg, err := config.Read(context.Background(), rutils.ResolveFile(relativePath), logger)
	test.That(t, err, test.ShouldBeNil)
	return testCfg
}

// readCaptureDir filters out folders from a slice of FileInfos.
func readCaptureDir(captureDir string) ([]fs.FileInfo, error) {
	filesAndFolders, err := ioutil.ReadDir(captureDir)
	var onlyFiles []fs.FileInfo
	for _, s := range filesAndFolders {
		if !s.IsDir() {
			onlyFiles = append(onlyFiles, s)
		}
	}
	return onlyFiles, err
}

func deleteFilesInDirectory(t *testing.T, dir string) {
	deleteDir, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Log(err)
	}
	for _, file := range deleteDir {
		os.RemoveAll(filepath.Join(dir, file.Name()))
	}
}

// Validates that manual syncing works for a datamanager.
func TestManualSync(t *testing.T) {
	configPath := "robots/configs/datamanager_fake.json"
	testCfg := setupConfig(t, configPath)
	captureDir := "/tmp/capture/arm/arm1/" // Make the captureDir where we're logging data for our arm.
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"

	// Clear the capture and queue dirs.
	deleteFilesInDirectory(t, captureDir)
	deleteFilesInDirectory(t, queueDir)
	defer deleteFilesInDirectory(t, queueDir)
	defer deleteFilesInDirectory(t, captureDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, testCfg, captureDir).(internal.Service)
	defer dmsvc.Close(context.Background())
	dmsvc.Update(context.Background(), testCfg)

	// Look at the files in captureDir.
	filesInCaptureDir, err := readCaptureDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	firstFileInCaptureDir := filesInCaptureDir[0].Name()
	// Give it a second to run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Second)

	//Verify files were enqueued and uploaded.
	filesInCaptureDir, err = readCaptureDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueueDir, err := ioutil.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}

	// We should have uploaded the first file and should now be collecting another one.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	secondFileInCaptureDir := filesInCaptureDir[0].Name()
	test.That(t, firstFileInCaptureDir, test.ShouldNotEqual, secondFileInCaptureDir)

	// TODO: Change to 0 once we fix the TODO in the syncer.Upload function to actually delete files once they're uploaded.
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 1)
}

// Validates that scheduled syncing works for a datamanager.
func TestScheduledSync(t *testing.T) {
	configPath := "robots/configs/datamanager_fake.json"
	testCfg := setupConfig(t, configPath)
	captureDir := "/tmp/capture/arm/arm1/" // Make the captureDir where we're logging data for our arm.
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	// Clear the capture and queue dirs.
	deleteFilesInDirectory(t, captureDir)
	deleteFilesInDirectory(t, queueDir)
	defer deleteFilesInDirectory(t, queueDir)
	defer deleteFilesInDirectory(t, captureDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, testCfg, captureDir).(internal.Service)
	// Tell the datamanager to close later.
	defer dmsvc.Close(cancelCtx)
	defer cancelFn()
	dmsvc.Update(cancelCtx, testCfg)

	// Look at the files in captureDir.
	filesInCaptureDir, err := readCaptureDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	firstFileInCaptureDir := filesInCaptureDir[0].Name()

	// Queue files and wait for operation to finish.
	dmsvc.QueueCapturedData(cancelCtx, 1000)
	time.Sleep(time.Millisecond * 1300)

	// Verify files were enqueued.
	filesInQueueDir, err := ioutil.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	// Should have 1 file in the queue since we just moved it there.
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 1)

	filesInCaptureDir, err = readCaptureDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// We should have uploaded the first file and should now be collecting another one.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	secondFileInCaptureDir := filesInCaptureDir[0].Name()
	test.That(t, firstFileInCaptureDir, test.ShouldNotEqual, secondFileInCaptureDir)

	// TODO: Change to 0 once we fix the TODO in the syncer.Upload function to actually delete files once they're uploaded.
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 1)
}

// Validates that we can attempt a scheduled and manual sync at the same time without duplicating files or running into errors.
func TestManualAndScheduledSync(t *testing.T) {
	configPath := "robots/configs/datamanager_fake.json"
	testCfg := setupConfig(t, configPath)
	captureDir := "/tmp/capture/arm/arm1/" // Make the captureDir where we're logging data for our arm.
	queueDir := datamanager.SyncQueuePath + "/arm/arm1/"
	cancelCtx, cancelFn := context.WithCancel(context.Background())

	// Clear the capture and queue dirs.
	deleteFilesInDirectory(t, captureDir)
	deleteFilesInDirectory(t, queueDir)
	defer deleteFilesInDirectory(t, queueDir)
	defer deleteFilesInDirectory(t, captureDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, testCfg, captureDir).(internal.Service)

	// Make sure we close resources to prevent leaks.
	defer dmsvc.Close(cancelCtx)
	defer cancelFn()
	dmsvc.Update(cancelCtx, testCfg)

	// Look at the files in captureDir.
	filesInCaptureDir, err := readCaptureDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// Since we have 1 collector, we should be expecting 1 file.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	firstFileInCaptureDir := filesInCaptureDir[0].Name()

	// Queue files and perform a manual sync at approximately the same time.
	dmsvc.QueueCapturedData(cancelCtx, 1000)
	time.Sleep(time.Millisecond * 1000)
	dmsvc.Sync(cancelCtx)

	time.Sleep(time.Millisecond * 500)
	// Verify two different files were enqueued.
	filesInQueueDir, err := ioutil.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}

	// Expect to have 2 files in the queue with different timestamps, since the enqueueing shouldn't enqueue the same file.
	test.That(t, len(filesInQueueDir), test.ShouldEqual, 2)
	test.That(t, filesInQueueDir[0].Name(), test.ShouldNotEqual, filesInQueueDir[1].Name())

	filesInCaptureDir, err = readCaptureDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}

	// We should have uploaded the first two files and should now be collecting another one.
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 1)
	secondFileInCaptureDir := filesInCaptureDir[0].Name()
	test.That(t, firstFileInCaptureDir, test.ShouldNotEqual, secondFileInCaptureDir)
}

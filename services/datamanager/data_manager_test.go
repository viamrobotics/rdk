package datamanager_test

import (
	"context"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
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

var (
	// 0.0041 mins is 246 milliseconds, this is the interval waiting time in the config file used for testing.
	configSyncIntervalMins = 0.0041

	// Make the captureDir where we're logging data for our arm.
	captureDir = "/tmp/capture"
	armDir     = captureDir + "/arm/arm1/GetEndPosition"
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
	testCfg.Cloud = &config.Cloud{ID: "part_id"}
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

	// Verify that the single configured collector wrote to its file.
	files, err := ioutil.ReadDir(svcConfig.CaptureDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 1)

	// Verify that after close is called, the collector is no longer writing.
	oldSize := files[0].Size()
	err = svc.Close(context.Background())
	// When Close returns all background processes in svc should be closed, but still sleep for 100ms to verify
	// that there's not a resource leak causing writes to still happens after Close() returns.
	time.Sleep(time.Millisecond * 100)
	test.That(t, err, test.ShouldBeNil)
	newSize := files[0].Size()
	test.That(t, oldSize, test.ShouldEqual, newSize)
}

// Validates that if the datamanager/robot die unexpectedly, that previously captured but not synced files are still
// synced at start up.
func TestRecoversAfterKilled(t *testing.T) {
	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}
	configPath := "robots/configs/fake_data_manager.json"
	testCfg := setupConfig(t, configPath)
	err := setconfigSyncIntervalMins(testCfg, configSyncIntervalMins)
	test.That(t, err, test.ShouldBeNil)

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/GetEndPosition"
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t)
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(context.TODO(), testCfg)

	// We set sync_interval_mins to be about 250ms in the config, so wait 150ms so data is captured but not synced.
	time.Sleep(time.Millisecond * 150)

	// Simulate turning off the service.
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)

	// Validate nothing has been synced yet.
	test.That(t, len(uploaded), test.ShouldEqual, 0)

	// Turn the service back on.
	dmsvc = newTestDataManager(t)
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(context.TODO(), testCfg)

	// Validate that the previously captured file was uploaded at startup.
	time.Sleep(time.Millisecond * 50)
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(uploaded), test.ShouldEqual, 1)
}

func setconfigSyncIntervalMins(config *config.Config, interval float64) error {
	svcConfig, ok, err := datamanager.GetServiceConfig(config)
	if !ok {
		return errors.New("failed to get service config")
	}
	if err != nil {
		return err
	}
	svcConfig.SyncIntervalMins = interval
	return nil
}

func setConfigAdditionalSyncPaths(config *config.Config, dirs []string) error {
	svcConfig, ok, err := datamanager.GetServiceConfig(config)
	if !ok {
		return errors.New("failed to get service config")
	}
	if err != nil {
		return err
	}
	svcConfig.AdditionalSyncPaths = dirs
	return nil
}

func populateArbitraryFiles(t *testing.T, configPath string, numDirs, maxFilesPerDir int) ([]string, *config.Config, int) {
	t.Helper()

	// Retrieve config from config filepath and
	testCfg := setupConfig(t, configPath)
	err := setconfigSyncIntervalMins(testCfg, configSyncIntervalMins)
	test.That(t, err, test.ShouldBeNil)

	// bytesPerFile value doesn't really matter, but maybe it will in future testing?
	bytesPerFile := 100

	// Slice of temp dirs to be generated.
	additionalSyncPaths := []string{}

	// Number of total files to be synced.
	numArbitraryFilesToSync := 0

	// Begin generating additional_sync_paths "dummy" dirs & files.
	for d := 0; d < numDirs; d++ {
		// Create a temp dir that will be in additional_sync_paths.
		td, err := ioutil.TempDir("", "additional_sync_path_dir_")
		if err != nil {
			t.Error("cannot create temporary dir to simulate additional_sync_paths in data manager service config")
		}
		additionalSyncPaths = append(additionalSyncPaths, td)

		// Make the first dir empty.
		if d == 0 {
			continue
		} else {
			// Make the dirs that will contain at least one (at most two) file(s).
			for i := 0; i < maxFilesPerDir; i++ {
				// Generate data that will be in a temp file.
				fileData := make([]byte, bytesPerFile)
				rand.Read(fileData)

				// Create arbitrary file that will be in the temp dir generated above.
				tf, err := ioutil.TempFile(td, "arbitrary_file_")
				if err != nil {
					t.Error("cannot create temporary file to simulate uploading from data manager service")
				}

				// Write data to the temp file.
				if _, err := tf.Write(fileData); err != nil {
					t.Error("cannot write arbitrary data to temporary file")
				}

				// Increment number of files to be synced.
				numArbitraryFilesToSync++
			}
		}
	}

	// Add the additional sync paths dirs to the test case datamanager config.
	err = setConfigAdditionalSyncPaths(testCfg, additionalSyncPaths)
	test.That(t, err, test.ShouldBeNil)

	return additionalSyncPaths, testCfg, numArbitraryFilesToSync
}

func noRepeatedElements(slice []string) bool {
	visited := make(map[string]bool, 0)
	for i := 0; i < len(slice); i++ {
		if visited[slice[i]] {
			return false
		}
		visited[slice[i]] = true
	}
	return true
}

// Validates that manual syncing works for a datamanager.
func TestManualSync(t *testing.T) {
	// Empty file structure at beginning of testing to ensure no files from previous tests remain
	resetFolder(t, captureDir)
	resetFolder(t, armDir)

	// Test Case Setup
	configPath := "robots/configs/fake_data_manager.json"
	numDirs := 2
	maxFilesPerDir := 2
	dirs, testCfg, numArbitraryFilesToSync := populateArbitraryFiles(t, configPath, numDirs, maxFilesPerDir)

	// Once testing is complete, remove the temp dirs created in [populateArbitraryFiles] and all files within.
	defer func() {
		for _, dir := range dirs {
			resetFolder(t, dir)
		}
	}()
	// Clear the capture dir after we're done.
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)

	// Prepare list uploaded filepaths.
	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partId string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t)
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(context.TODO(), testCfg)

	// Run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Millisecond * 100)

	// Verify that one data capture files was uploaded, two additional_sync_paths files were uploaded,
	// and that no two uploaded files are the same.
	lock.Lock()
	test.That(t, len(uploaded), test.ShouldEqual, (numArbitraryFilesToSync + 1))
	test.That(t, noRepeatedElements(uploaded), test.ShouldBeTrue)
	lock.Unlock()

	// Sync again and verify it synced the second file data capture file, but not the first again.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Millisecond * 100)
	_ = dmsvc.Close(context.TODO())
	test.That(t, len(uploaded), test.ShouldEqual, (numArbitraryFilesToSync + 2))
	test.That(t, noRepeatedElements(uploaded), test.ShouldBeTrue)
}

// Validates that scheduled syncing works for a datamanager.
func TestScheduledSync(t *testing.T) {
	// Empty file structure at beginning of testing to ensure no files from previous tests remain
	resetFolder(t, captureDir)
	resetFolder(t, armDir)

	// Test Case Setup
	configPath := "robots/configs/fake_data_manager.json"
	numDirs := 2
	maxFilesPerDir := 2
	dirs, testCfg, numArbitraryFilesToSync := populateArbitraryFiles(t, configPath, numDirs, maxFilesPerDir)

	// Once testing is complete, remove the temp dirs created in [populateArbitraryFiles] and all files within.
	defer func() {
		for _, dir := range dirs {
			os.RemoveAll(dir)
		}
	}()
	// Clear the capture dir after we're done.
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)

	// Prepare list uploaded filepaths.
	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t)
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(context.TODO(), testCfg)

	// We set sync_interval_mins to be about 250ms in the config, so wait 600ms (more than two iterations of syncing)
	// for the additional_sync_paths files to sync AND for TWO data capture files to sync.
	time.Sleep(time.Millisecond * 600)
	_ = dmsvc.Close(context.TODO())

	// Verify that the additional_sync_paths files AND the TWO data capture files were uploaded.
	lock.Lock()
	test.That(t, len(uploaded), test.ShouldEqual, (numArbitraryFilesToSync + 2))
	test.That(t, noRepeatedElements(uploaded), test.ShouldBeTrue)
	lock.Unlock()
}

// Validates that we can attempt a scheduled and manual syncDataCaptureFiles at the same time without duplicating files
// or running into errors.
func TestManualAndScheduledSync(t *testing.T) {
	// Empty file structure at beginning of testing to ensure no files from previous tests remain
	resetFolder(t, captureDir)
	resetFolder(t, armDir)

	// Test Case Setup
	configPath := "robots/configs/fake_data_manager.json"
	numDirs := 2
	maxFilesPerDir := 2
	dirs, testCfg, numArbitraryFilesToSync := populateArbitraryFiles(t, configPath, numDirs, maxFilesPerDir)

	// Clear the capture dir after we're done.
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)

	// Once testing is complete, remove the temp dirs created in [populateArbitraryFiles] and all files within.
	defer func() {
		for _, dir := range dirs {
			os.RemoveAll(dir)
		}
	}()

	// Prepare list uploaded filepaths.
	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t)
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.Update(context.TODO(), testCfg)

	// Perform a manual and scheduled syncDataCaptureFiles at approximately the same time, then close the svc.
	time.Sleep(time.Millisecond * 250)
	dmsvc.Sync(context.TODO())
	time.Sleep(time.Millisecond * 100)
	_ = dmsvc.Close(context.TODO())

	// Verify that two data capture files were uploaded, two additional_sync_paths files were uploaded,
	// and that no two uploaded files are the same.
	lock.Lock()
	test.That(t, len(uploaded), test.ShouldEqual, (numArbitraryFilesToSync + 2))
	test.That(t, noRepeatedElements(uploaded), test.ShouldBeTrue)
	lock.Unlock()

	// We've uploaded the first two files and should now be collecting a single new one.
	filesInArmDir, err := readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}
	test.That(t, len(filesInArmDir), test.ShouldEqual, 3)
}

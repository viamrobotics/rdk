package datamanager_test

import (
	"context"
	"io/fs"
	"io/ioutil"
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
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

var (
	// Robot config which specifies data manager service.
	configPath = "robots/configs/fake_data_manager.json"

	// 0.0041 mins is 246 milliseconds, this is the interval waiting time in the config file used for testing.
	configSyncIntervalMins = 0.0041

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

func getInjectedRobotWithArm(armKey string) *inject.Robot {
	r := &inject.Robot{}
	rs := map[resource.Name]interface{}{}
	injectedArm := &inject.Arm{}
	injectedArm.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return &commonpb.Pose{X: 1, Y: 2, Z: 3}, nil
	}
	rs[arm.Named(armKey)] = injectedArm
	r.MockResourcesFromMap(rs)
	return r
}

func newTestDataManager(t *testing.T, localArmKey string, remoteArmKey string) internal.DMService {
	t.Helper()
	dmCfg := &datamanager.Config{}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: dmCfg,
	}
	logger := golog.NewTestLogger(t)

	// Create local robot with injected arm.
	r := getInjectedRobotWithArm(localArmKey)

	// If passed, create remote robot with an injected arm.
	if remoteArmKey != "" {
		remoteRobot := getInjectedRobotWithArm(remoteArmKey)

		r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
			return remoteRobot, true
		}
	}

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
	svc := newTestDataManager(t, "arm1", "")
	// Set capture parameters in Update.
	conf := setupConfig(t, configPath)
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

func TestNewRemoteDataManager(t *testing.T) {
	// Empty config at initialization.
	svc := newTestDataManager(t, "localArm", "remoteArm")

	// Set capture parameters in Update.
	conf := setupConfig(t, "robots/configs/fake_robot_with_remote_and_data_manager.json")
	defer resetFolder(t, captureDir)
	svc.Update(context.Background(), conf)
	sleepTime := time.Millisecond * 100
	time.Sleep(sleepTime)

	// Verify that after close is called, the collector is no longer writing.
	err := svc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Verify that the local and remote collectors wrote to their files.
	localArmDir := captureDir + "/arm/localArm/GetEndPosition"
	filesInLocalArmDir, err := readDir(t, localArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInLocalArmDir), test.ShouldEqual, 1)
	test.That(t, filesInLocalArmDir[0].Size(), test.ShouldBeGreaterThan, 0)

	remoteArmDir := captureDir + "/arm/remoteArm/GetEndPosition"
	filesInRemoteArmDir, err := readDir(t, remoteArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInRemoteArmDir), test.ShouldEqual, 1)
	test.That(t, filesInRemoteArmDir[0].Size(), test.ShouldBeGreaterThan, 0)
}

// Validates that if the datamanager/robot die unexpectedly, that previously captured but not synced files are still
// synced at start up.
func TestRecoversAfterKilled(t *testing.T) {
	dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
	defer func() {
		for _, dir := range dirs {
			resetFolder(t, dir)
		}
	}()
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)
	if err != nil {
		t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
	}

	testCfg := setupConfig(t, configPath)
	err = setconfigSyncIntervalMins(testCfg, configSyncIntervalMins)
	test.That(t, err, test.ShouldBeNil)
	err = setConfigAdditionalSyncPaths(testCfg, dirs)
	test.That(t, err, test.ShouldBeNil)

	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partId string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.SetWaitAfterLastModifiedSecs(10)
	dmsvc.Update(context.TODO(), testCfg)

	// We set sync_interval_mins to be about 250ms in the config, so wait 150ms so data is captured but not synced.
	time.Sleep(time.Millisecond * 150)

	// Simulate turning off the service.
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)

	// Validate nothing has been synced yet.
	test.That(t, len(uploaded), test.ShouldEqual, 0)

	// Turn the service back on.
	dmsvc = newTestDataManager(t, "arm1", "")
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	dmsvc.Update(context.TODO(), testCfg)

	// Validate that the previously captured file was uploaded at startup.
	time.Sleep(time.Millisecond * 50)
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(uploaded), test.ShouldEqual, (1 + numArbitraryFilesToSync))
}

// Validates that if the robot config file specifies a directory path in additionalSyncPaths that does not exist,
// that directory is created (and can be synced on subsequent iterations of syncing).
func TestCreatesAdditionalSyncPaths(t *testing.T) {
	td := "additional_sync_path_dir"
	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}
	// Once testing is complete, remove contents from data capture dirs.
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)
	defer resetFolder(t, td)

	testCfg := setupConfig(t, configPath)
	err := setconfigSyncIntervalMins(testCfg, configSyncIntervalMins)
	test.That(t, err, test.ShouldBeNil)

	// Add a directory path to additionalSyncPaths that does not exist. The data manager should create it.
	err = setConfigAdditionalSyncPaths(testCfg, []string{td})
	test.That(t, err, test.ShouldBeNil)

	// Initialize the data manager and update it with our config. The call to Update(ctx, conf) should create the
	// arbitrary sync paths directory it in the file system.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	dmsvc.Update(context.TODO(), testCfg)

	// Validate the "additional_sync_path_dir" was created. Wait some time to ensure it would have been created.
	time.Sleep(time.Millisecond * 100)
	_ = dmsvc.Close(context.TODO())
	_, err = os.Stat(td)
	test.That(t, errors.Is(err, nil), test.ShouldBeTrue)
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

// Generates and populates a directory structure of files that contain arbitrary file data. Used to simulate testing
// syncing of data in the service's additional_sync_paths.
//nolint
func populateAdditionalSyncPaths() ([]string, int, error) {
	additionalSyncPaths := []string{}
	numArbitraryFilesToSync := 0

	// Generate additional_sync_paths "dummy" dirs & files.
	for i := 0; i < 2; i++ {
		// Create a temp dir that will be in additional_sync_paths.
		td, err := ioutil.TempDir("", "additional_sync_path_dir_")
		if err != nil {
			return []string{}, 0, errors.New("cannot create temporary dir to simulate additional_sync_paths in data manager service config")
		}
		additionalSyncPaths = append(additionalSyncPaths, td)

		// Make the first dir empty.
		if i == 0 {
			continue
		} else {
			// Make the dirs that will contain two file.
			for i := 0; i < 2; i++ {
				// Generate data that will be in a temp file.
				fileData := []byte("This is file data. It will be stored in a directory included in the user's specified additional sync paths. Hopefully it is uploaded from the robot to the cloud!")

				// Create arbitrary file that will be in the temp dir generated above.
				tf, err := ioutil.TempFile(td, "arbitrary_file_")
				if err != nil {
					return nil, 0, errors.New("cannot create temporary file to simulate uploading from data manager service")
				}

				// Write data to the temp file.
				if _, err := tf.Write(fileData); err != nil {
					return nil, 0, errors.New("cannot write arbitrary data to temporary file")
				}

				// Increment number of files to be synced.
				numArbitraryFilesToSync++
			}
		}
	}
	return additionalSyncPaths, numArbitraryFilesToSync, nil
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
	dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
	defer func() {
		for _, dir := range dirs {
			resetFolder(t, dir)
		}
	}()
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)
	if err != nil {
		t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
	}
	testCfg := setupConfig(t, configPath)
	err = setconfigSyncIntervalMins(testCfg, configSyncIntervalMins)
	test.That(t, err, test.ShouldBeNil)
	err = setConfigAdditionalSyncPaths(testCfg, dirs)
	test.That(t, err, test.ShouldBeNil)

	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partId string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	dmsvc.Update(context.TODO(), testCfg)

	// Run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Millisecond * 100)

	// Verify that one data capture file was uploaded, two additional_sync_paths files were uploaded,
	// and that no two uploaded files are the same.
	lock.Lock()
	test.That(t, len(uploaded), test.ShouldEqual, (numArbitraryFilesToSync + 1))
	test.That(t, noRepeatedElements(uploaded), test.ShouldBeTrue)
	lock.Unlock()

	// Sync again and verify it synced the second data capture file, but also validate that it didn't attempt to resync
	// any files that were previously synced.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Millisecond * 100)
	_ = dmsvc.Close(context.TODO())
	test.That(t, len(uploaded), test.ShouldEqual, (numArbitraryFilesToSync + 2))
	test.That(t, noRepeatedElements(uploaded), test.ShouldBeTrue)
}

// Validates that scheduled syncing works for a datamanager.
func TestScheduledSync(t *testing.T) {
	dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
	defer func() {
		for _, dir := range dirs {
			os.RemoveAll(dir)
		}
	}()
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)
	if err != nil {
		t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
	}
	testCfg := setupConfig(t, configPath)
	err = setconfigSyncIntervalMins(testCfg, configSyncIntervalMins)
	test.That(t, err, test.ShouldBeNil)
	err = setConfigAdditionalSyncPaths(testCfg, dirs)
	test.That(t, err, test.ShouldBeNil)

	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.SetWaitAfterLastModifiedSecs(0)
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
	dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
	defer func() {
		for _, dir := range dirs {
			resetFolder(t, dir)
		}
	}()
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)
	if err != nil {
		t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
	}
	testCfg := setupConfig(t, configPath)
	err = setconfigSyncIntervalMins(testCfg, configSyncIntervalMins)
	test.That(t, err, test.ShouldBeNil)
	err = setConfigAdditionalSyncPaths(testCfg, dirs)
	test.That(t, err, test.ShouldBeNil)

	uploaded := []string{}
	lock := sync.Mutex{}
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
		lock.Lock()
		uploaded = append(uploaded, path)
		lock.Unlock()
		return nil
	}

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetUploadFn(uploadFn)
	dmsvc.SetWaitAfterLastModifiedSecs(0)
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

	// We've uploaded (and thus deleted) the first two files and should now be collecting a single new one.
	filesInArmDir, err := readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
}

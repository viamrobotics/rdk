package builtin

import (
	"context"
	"fmt"
	"image"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	captureWaitTime = time.Millisecond * 25
	syncWaitTime    = time.Millisecond * 100
)

var (
	// Robot config which specifies data manager service.
	configPath = "services/datamanager/data/data_capture_config.json"

	captureDir         = "/tmp/capture"
	armDir             = captureDir + "/arm/arm1/GetEndPosition"
	emptyFileBytesSize = 30 // size of leading metadata message
)

// readDir filters out folders from a slice of FileInfos.
func readDir(t *testing.T, dir string) ([]fs.DirEntry, error) {
	t.Helper()
	filesAndFolders, err := os.ReadDir(dir)
	if err != nil {
		t.Log(err)
	}
	var onlyFiles []fs.DirEntry
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
	injectedArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (*commonpb.Pose, error) {
		return &commonpb.Pose{X: 1, Y: 2, Z: 3}, nil
	}
	rs[arm.Named(armKey)] = injectedArm
	r.MockResourcesFromMap(rs)
	return r
}

func getInjectedRobotWithCamera(t *testing.T) *inject.Robot {
	t.Helper()
	r := &inject.Robot{}
	rs := map[resource.Name]interface{}{}

	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	injectCamera := &inject.Camera{}
	var imageReleasedMu sync.Mutex
	injectCamera.StreamFunc = func(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
		return gostream.NewEmbeddedVideoStreamFromReader(gostream.VideoReaderFunc(func(ctx context.Context) (image.Image, func(), error) {
			imageReleasedMu.Lock()
			time.Sleep(10 * time.Nanosecond)
			imageReleasedMu.Unlock()
			return img, func() {}, nil
		})), nil
	}

	rs[camera.Named("c1")] = injectCamera
	r.MockResourcesFromMap(rs)
	return r
}

func newTestDataManager(t *testing.T, localArmKey, remoteArmKey string) internal.DMService {
	t.Helper()
	dmCfg := &Config{}
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

	svc, err := NewBuiltIn(context.Background(), r, cfgService, logger)
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

/**
TODO things:
- Figure out how to programmatically edit things in config. Would be VERY helpful in many places to be able to manually
  set things, e.g. disabled, capture directory, etc. Think can do this by editing svcconfig

*/

/**
New test setup!
First data capture only tests:
- Captures what we'd expect. Both contents and amount, to where we expect it.
- Disabling capture stops it. Re-enabling re-begins it.

*/

func TestDataCapture(t *testing.T) {
	captureTime := time.Millisecond * 25

	tests := []struct {
		name                 string
		initialDisableStatus bool
		newDisableStatus     bool
	}{
		{
			"Config with data capture disabled should capture nothing.",
			true,
			true,
		},
		{
			"Config with data capture enabled should capture data.",
			false,
			false,
		},
		{
			"Disabling data capture should cause it to stop.",
			false,
			true,
		},
		{
			"Enabling data capture should cause it to begin.",
			true,
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "")
			test.That(t, err, test.ShouldBeNil)

			rpcServer, _ := buildAndStartLocalServer(t)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()
			dmsvc := newTestDataManager(t, "arm1", "")
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
			testCfg := setupConfig(t, configPath)
			svcConfig, ok, err := getServiceConfig(testCfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok, test.ShouldBeTrue)
			svcConfig.CaptureDisabled = tc.initialDisableStatus
			svcConfig.ScheduledSyncDisabled = true
			svcConfig.CaptureDir = tmpDir
			err = dmsvc.Update(context.Background(), testCfg)

			// Let run for a second, then change status.
			time.Sleep(captureTime)
			svcConfig.CaptureDisabled = tc.newDisableStatus
			err = dmsvc.Update(context.Background(), testCfg)

			// Check if data has been captured (or not) as we'd expect.
			initialCaptureFiles := getAllFiles(tmpDir)
			if !tc.initialDisableStatus {
				// TODO: check contents
				test.That(t, len(initialCaptureFiles), test.ShouldBeGreaterThan, 0)
			} else {
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
			}

			// Let run for a second.
			time.Sleep(captureTime)
			// Check if data has been captured (or not) as we'd expect.
			updatedCaptureFiles := getAllFiles(tmpDir)
			if !tc.newDisableStatus {
				// TODO: check contents
				test.That(t, len(updatedCaptureFiles), test.ShouldBeGreaterThan, len(initialCaptureFiles))
			} else {
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, len(initialCaptureFiles))
			}
			test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
		})
	}
}

/**
Now sync tests
- Enabling and disabling work.
- If already captured data is there on start up, it syncs it.
- Already captured data sync and new data sync don't conflict/can occur simultaneously.
*/
func TestSync(t *testing.T) {
	//tests := []struct {
	//	name                 string
	//	initialDisableStatus bool
	//	newDisableStatus     bool
	//	alreadyCapturedData  bool
	//}{
	//	{
	//		"Config with sync disabled should sync nothing.",
	//		false,
	//		false,
	//		false,
	//	},
	//	{
	//		"Config with sync enabled should sync data.",
	//		false,
	//		false,
	//		false,
	//	},
	//	{
	//		"Disabling sync should cause it to stop.",
	//		false,
	//		false,
	//		false,
	//	},
	//	{
	//		"Enabling sync should cause it to begin.",
	//		false,
	//		false,
	//		false,
	//	},
	//	{
	//		"Already captured data should be synced at start up.",
	//		false,
	//		false,
	//		false,
	//	},
	//}
}

func TestNewDataManager(t *testing.T) {
	// Empty config at initialization.
	captureDir := "/tmp/capture"
	defer resetFolder(t, captureDir)
	fmt.Println("updating")
	resetFolder(t, captureDir)
	rpcServer, _ := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	testCfg := setupConfig(t, configPath)

	err := dmsvc.Update(context.Background(), testCfg)
	fmt.Println("done updating")
	test.That(t, err, test.ShouldBeNil)
	captureTime := time.Millisecond * 100
	time.Sleep(captureTime)

	fmt.Println("closing dm service")
	err = dmsvc.Close(context.Background())
	fmt.Println("closed dm service")
	test.That(t, err, test.ShouldBeNil)

	// Check that a collector wrote to file.
	armDir := captureDir + "/arm/arm1/GetEndPosition"
	filesInArmDir, err := readDir(t, armDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
	oldInfo, err := filesInArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	oldSize := oldInfo.Size()
	test.That(t, oldSize, test.ShouldBeGreaterThan, emptyFileBytesSize)

	// Check that dummy tags "a" and "b" are being wrote to metadata.
	captureFileName := filesInArmDir[0].Name()
	file, err := os.Open(armDir + "/" + captureFileName)
	test.That(t, err, test.ShouldBeNil)
	captureFile, err := datacapture.NewFileFromFile(file)
	test.That(t, err, test.ShouldBeNil)
	md, err := captureFile.ReadMetadata()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, md.Tags[0], test.ShouldEqual, "a")
	test.That(t, md.Tags[1], test.ShouldEqual, "b")

	// When Close returns all background processes in svc should be closed, but still sleep for 100ms to verify
	// that there's not a resource leak causing writes to still happens after Close() returns.
	time.Sleep(captureTime)
	test.That(t, err, test.ShouldBeNil)
	filesInArmDir, err = readDir(t, armDir)
	test.That(t, err, test.ShouldBeNil)
	newInfo, err := filesInArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	newSize := newInfo.Size()
	test.That(t, oldSize, test.ShouldEqual, newSize)
}

func TestCaptureDisabled(t *testing.T) {
	// Empty config at initialization.
	captureDir := "/tmp/capture"
	dmsvc := newTestDataManager(t, "arm1", "")
	// Set capture parameters in Update.
	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)

	defer resetFolder(t, captureDir)
	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(captureWaitTime)

	// Call Update with a disabled capture and give the collector time to write to file.
	dmCfg.CaptureDisabled = true
	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(captureWaitTime)

	// Verify that the collector wrote to its file.
	armDir := captureDir + "/arm/arm1/GetEndPosition"
	filesInArmDir, err := readDir(t, armDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
	info, err := filesInArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, emptyFileBytesSize)

	// Re-enable capture.
	dmCfg.CaptureDisabled = false
	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(captureWaitTime)

	// Close service.
	err = dmsvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Verify that started collection began in a new file when it was re-enabled.
	filesInArmDir, err = readDir(t, armDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInArmDir), test.ShouldEqual, 2)

	// Verify that something different was written to both files.
	test.That(t, filesInArmDir[0], test.ShouldNotEqual, filesInArmDir[1])
	info, err = filesInArmDir[1].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, emptyFileBytesSize)
}

func TestNewRemoteDataManager(t *testing.T) {
	// Empty config at initialization.
	captureDir := "/tmp/capture"
	dmsvc := newTestDataManager(t, "localArm", "remoteArm")

	// Set capture parameters in Update.
	conf := setupConfig(t, "services/datamanager/data/data_capture_remote_config.json")
	defer resetFolder(t, captureDir)
	err := dmsvc.Update(context.Background(), conf)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(captureWaitTime)

	// Verify that after close is called, the collector is no longer writing.
	err = dmsvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Verify that the local and remote collectors wrote to their files.
	localArmDir := captureDir + "/arm/localArm/GetEndPosition"
	filesInLocalArmDir, err := readDir(t, localArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInLocalArmDir), test.ShouldEqual, 1)
	info, err := filesInLocalArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, 0)

	remoteArmDir := captureDir + "/arm/remoteArm/GetEndPosition"
	filesInRemoteArmDir, err := readDir(t, remoteArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInRemoteArmDir), test.ShouldEqual, 1)
	info, err = filesInRemoteArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, 0)
}

// Validates that if the datamanager/robot die unexpectedly, that previously captured but not synced files are still
// synced at start up.
func TestRecoversAfterKilled(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

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
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.AdditionalSyncPaths = dirs

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(10)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// We set sync_interval_mins to be about 250ms in the config, so wait 150ms so data is captured but not synced.
	time.Sleep(time.Millisecond * 150)

	// Simulate turning off the service.
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)

	// Validate nothing has been synced yet.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)

	// Turn the service back on.
	dmsvc = newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Validate that the previously captured file was uploaded at startup.
	time.Sleep(syncWaitTime)
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 1+numArbitraryFilesToSync)
}

// Validates that if the robot config file specifies a directory path in additionalSyncPaths that does not exist,
// that directory is created (and can be synced on subsequent iterations of syncing).
func TestCreatesAdditionalSyncPaths(t *testing.T) {
	td := "additional_sync_path_dir"
	// Once testing is complete, remove contents from data capture dirs.
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)
	defer resetFolder(t, td)

	// Register mock datasync service with a mock server.
	rpcServer, _ := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.AdditionalSyncPaths = []string{td}

	// Initialize the data manager and update it with our config. The call to Update(ctx, conf) should create the
	// arbitrary sync paths directory it in the file system.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Validate the "additional_sync_path_dir" was created. Wait some time to ensure it would have been created.
	time.Sleep(syncWaitTime)
	_ = dmsvc.Close(context.TODO())
	_, err = os.Stat(td)
	test.That(t, errors.Is(err, nil), test.ShouldBeTrue)
}

// Generates and populates a directory structure of files that contain arbitrary file data. Used to simulate testing
// syncing of data in the service's additional_sync_paths.
// nolint
func populateAdditionalSyncPaths() ([]string, int, error) {
	var additionalSyncPaths []string
	numArbitraryFilesToSync := 0

	// Generate additional_sync_paths "dummy" dirs & files.
	for i := 0; i < 2; i++ {
		// Create a temp dir that will be in additional_sync_paths.
		td, err := os.MkdirTemp("", "additional_sync_path_dir_")
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
				tf, err := os.CreateTemp(td, "arbitrary_file_")
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
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

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
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.AdditionalSyncPaths = dirs

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Run and upload files.
	err = dmsvc.Sync(context.Background())
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(syncWaitTime)

	// Verify that one data capture file was uploaded, two additional_sync_paths files were uploaded,
	// and that no two uploaded files are the same.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+1)
	test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)

	// Sync again and verify it synced the second data capture file, but also validate that it didn't attempt to resync
	// any files that were previously synced.
	err = dmsvc.Sync(context.Background())
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(syncWaitTime)
	_ = dmsvc.Close(context.TODO())
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+2)
	test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)
}

// Validates that scheduled syncing works for a datamanager.
func TestScheduledSync(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
	defer func() {
		for _, dir := range dirs {
			_ = os.RemoveAll(dir)
		}
	}()
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)
	if err != nil {
		t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
	}
	// Use config with 250ms sync interval.
	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.AdditionalSyncPaths = dirs

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/GetEndPosition"

	// Clear the capture dir after we're done.
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// We set sync_interval_mins to be about 250ms in the config, so wait 600ms (more than two iterations of syncing)
	// for the additional_sync_paths files to sync AND for TWO data capture files to sync.
	time.Sleep(time.Millisecond * 600)
	_ = dmsvc.Close(context.TODO())

	// Verify that the additional_sync_paths files AND the TWO data capture files were uploaded.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+2)
	test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)
}

// Validates that we can attempt a scheduled and manual syncDataCaptureFiles at the same time without duplicating files
// or running into errors.
func TestManualAndScheduledSync(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

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
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.AdditionalSyncPaths = dirs

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/GetEndPosition"
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Perform a manual and scheduled syncDataCaptureFiles at approximately the same time, then close the svc.
	time.Sleep(time.Millisecond * 250)
	err = dmsvc.Sync(context.TODO())
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(syncWaitTime)
	_ = dmsvc.Close(context.TODO())

	// Verify that two data capture files were uploaded, two additional_sync_paths files were uploaded,
	// and that no two uploaded files are the same.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+2)
	test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)

	// We've uploaded (and thus deleted) the first two files and should now be collecting a single new one.
	filesInArmDir, err := readDir(t, armDir)
	if err != nil {
		t.Fatalf("failed to list files in armDir")
	}
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
}

func TestSyncDisabled(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	resetFolder(t, captureDir)
	defer resetFolder(t, captureDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// We set sync_interval_mins to be about 250ms in the config, so wait 150ms so data is captured but not synced.
	time.Sleep(time.Millisecond * 150)

	// Simulate disabling sync.
	dmCfg.ScheduledSyncDisabled = true
	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Validate nothing has been synced yet.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)

	// Re-enable sync.
	dmCfg.ScheduledSyncDisabled = false
	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// We set sync_interval_mins to be about 250ms in the config, so wait 600ms and ensure three files were uploaded:
	// one from file immediately uploaded when sync was re-enabled and two after.
	time.Sleep(time.Millisecond * 600)
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 3)
}

func TestGetDurationFromHz(t *testing.T) {
	test.That(t, GetDurationFromHz(0.1), test.ShouldEqual, time.Second*10)
	test.That(t, GetDurationFromHz(0.5), test.ShouldEqual, time.Second*2)
	test.That(t, GetDurationFromHz(1), test.ShouldEqual, time.Second)
	test.That(t, GetDurationFromHz(1000), test.ShouldEqual, time.Millisecond)
	test.That(t, GetDurationFromHz(0), test.ShouldEqual, 0)
}

func TestAdditionalParamsInConfig(t *testing.T) {
	conf := setupConfig(t, "services/datamanager/data/robot_with_cam_capture.json")
	r := getInjectedRobotWithCamera(t)

	dmCfg := &Config{}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: dmCfg,
	}
	logger := golog.NewTestLogger(t)
	svc, err := NewBuiltIn(context.Background(), r, cfgService, logger)
	if err != nil {
		t.Log(err)
	}

	dmsvc := svc.(internal.DMService)

	defer resetFolder(t, captureDir)

	err = dmsvc.Update(context.Background(), conf)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(captureWaitTime)

	filesInCamDir, err := readDir(t, captureDir+"/camera/c1/Next")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInCamDir), test.ShouldEqual, 1)
	info, err := filesInCamDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, emptyFileBytesSize)

	// Verify that after close is called, the collector is no longer writing.
	err = dmsvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
	err = r.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func getDataManagerConfig(config *config.Config) (*Config, error) {
	svcConfig, ok, err := GetServiceConfig(config)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("failed to get service config")
	}
	return svcConfig, nil
}

type mockDataSyncServiceServer struct {
	uploadedFiles *[]string
	lock          *sync.Mutex
	v1.UnimplementedDataSyncServiceServer
}

func (m mockDataSyncServiceServer) getUploadedFiles() []string {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	return *m.uploadedFiles
}

func (m mockDataSyncServiceServer) Upload(stream v1.DataSyncService_UploadServer) error {
	var fileName string
	for {
		ur, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if ur.GetMetadata() != nil {
			fileName = ur.GetMetadata().GetFileName()
		}
	}
	(*m.lock).Lock()
	*m.uploadedFiles = append(*m.uploadedFiles, fileName)
	(*m.lock).Unlock()
	return nil
}

//nolint:thelper
func buildAndStartLocalServer(t *testing.T) (rpc.Server, *mockDataSyncServiceServer) {
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	mockService := mockDataSyncServiceServer{
		uploadedFiles:                      &[]string{},
		lock:                               &sync.Mutex{},
		UnimplementedDataSyncServiceServer: v1.UnimplementedDataSyncServiceServer{},
	}
	err = rpcServer.RegisterServiceServer(
		context.Background(),
		&v1.DataSyncService_ServiceDesc,
		mockService,
		v1.RegisterDataSyncServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)

	// Stand up the server. Defer stopping the server.
	go func() {
		err := rpcServer.Start()
		test.That(t, err, test.ShouldBeNil)
	}()
	return rpcServer, &mockService
}

func getLocalServerConn(rpcServer rpc.Server, logger golog.Logger) (rpc.ClientConn, error) {
	return rpc.DialDirectGRPC(
		context.Background(),
		rpcServer.InternalAddr().String(),
		logger,
		rpc.WithInsecure(),
	)
}

//nolint:thelper
func getTestSyncerConstructor(t *testing.T, server rpc.Server) datasync.ManagerConstructor {
	return func(logger golog.Logger, cfg *config.Config) (datasync.Manager, error) {
		conn, err := getLocalServerConn(server, logger)
		test.That(t, err, test.ShouldBeNil)
		client := datasync.NewClient(conn)
		return datasync.NewManager(logger, cfg.Cloud.ID, client, conn)
	}
}

func getAllFiles(dir string) []os.FileInfo {
	var files []os.FileInfo
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, info)
		return nil
	})
	return files
}

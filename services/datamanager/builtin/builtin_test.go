package builtin

import (
	"context"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
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
	configPath = "services/datamanager/data/fake_robot_with_data_manager.json"

	// 0.0041 mins is 246 milliseconds, this is the interval waiting time in the config file used for testing.
	configSyncIntervalMins = 0.0041

	syncIntervalMins   = 0.0041 // 250ms
	captureDir         = "/tmp/capture"
	armDir             = captureDir + "/arm/arm1/EndPosition"
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

// TODO: get rid of this and just use os.RemoveAll where needed
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

	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
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

// TODO: determine if this can be removed. I think it can. It doesn't seem to test anything not covered by other tests.
//func TestNewDataManager(t *testing.T) {
//	dmsvc := newTestDataManager(t, "arm1", "")
//	testCfg := setupConfig(t, configPath)
//
//	// Empty config at initialization.
//	captureDir := "/tmp/capture"
//	defer resetFolder(t, captureDir)
//	resetFolder(t, captureDir)
//	err := dmsvc.Update(context.Background(), testCfg)
//	test.That(t, err, test.ShouldBeNil)
//	captureTime := time.Millisecond * 100
//	time.Sleep(captureTime)
//
//	err = dmsvc.Close(context.Background())
//	test.That(t, err, test.ShouldBeNil)
//
//	// Check that a collector wrote to file.
//	armDir := captureDir + "/arm/arm1/EndPosition"
//	filesInArmDir, err := readDir(t, armDir)
//	test.That(t, err, test.ShouldBeNil)
//	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
//	oldInfo, err := filesInArmDir[0].Info()
//	test.That(t, err, test.ShouldBeNil)
//	oldSize := oldInfo.Size()
//	test.That(t, oldSize, test.ShouldBeGreaterThan, emptyFileBytesSize)
//
//	// Check that dummy tags "a" and "b" are being wrote to metadata.
//	captureFileName := filesInArmDir[0].Name()
//	file, err := os.Open(armDir + "/" + captureFileName)
//	test.That(t, err, test.ShouldBeNil)
//	f, err := datacapture.ReadFile(file)
//	test.That(t, err, test.ShouldBeNil)
//	md := f.ReadMetadata()
//	test.That(t, md.Tags[0], test.ShouldEqual, "a")
//	test.That(t, md.Tags[1], test.ShouldEqual, "b")
//
//	// When Close returns all background processes in svc should be closed, but still sleep for 100ms to verify
//	// that there's not a resource leak causing writes to still happens after Close() returns.
//	time.Sleep(captureTime)
//	test.That(t, err, test.ShouldBeNil)
//	filesInArmDir, err = readDir(t, armDir)
//	test.That(t, err, test.ShouldBeNil)
//	newInfo, err := filesInArmDir[0].Info()
//	test.That(t, err, test.ShouldBeNil)
//	newSize := newInfo.Size()
//	test.That(t, oldSize, test.ShouldEqual, newSize)
//}

// TODO: add sync testing here too
func TestNewRemoteDataManager(t *testing.T) {
	// Empty config at initialization.
	captureDir := "/tmp/capture"
	dmsvc := newTestDataManager(t, "localArm", "remoteArm")

	// Set capture parameters in Update.
	conf := setupConfig(t, "services/datamanager/data/fake_robot_with_remote_and_data_manager.json")
	defer resetFolder(t, captureDir)
	err := dmsvc.Update(context.Background(), conf)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(captureWaitTime)

	// Verify that after close is called, the collector is no longer writing.
	err = dmsvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Verify that the local and remote collectors wrote to their files.
	localArmDir := captureDir + "/arm/localArm/EndPosition"
	filesInLocalArmDir, err := readDir(t, localArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInLocalArmDir), test.ShouldEqual, 1)
	info, err := filesInLocalArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, 0)

	remoteArmDir := captureDir + "/arm/remoteArm/EndPosition"
	filesInRemoteArmDir, err := readDir(t, remoteArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInRemoteArmDir), test.ShouldEqual, 1)
	info, err = filesInRemoteArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, 0)
}

// TODO: test that data capture files are resumed at start too
// Validates that if the datamanager/robot die unexpectedly, that previously captured but not synced files are still
// synced at start up.
func TestRecoversAfterKilled(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalSyncServer(t)
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
	dmCfg.SyncIntervalMins = configSyncIntervalMins
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

// TODO: replace with two more cases in TestDataCaptureEnabled
//func TestCollectorDisabled(t *testing.T) {
//	// Register mock datasync service with a mock server.
//	rpcServer, mockService := buildAndStartLocalSyncServer(t)
//	defer func() {
//		err := rpcServer.Stop()
//		test.That(t, err, test.ShouldBeNil)
//		resetFolder(t, captureDir)
//		resetFolder(t, armDir)
//	}()
//	defer resetFolder(t, captureDir)
//	defer resetFolder(t, armDir)
//
//	disabledCollectorConfigPath := "services/datamanager/data/fake_robot_with_disabled_collector.json"
//	testCfg := setupConfig(t, disabledCollectorConfigPath)
//	dmCfg, err := getDataManagerConfig(testCfg)
//	test.That(t, err, test.ShouldBeNil)
//
//	// Initialize the data manager and update it with our config.
//	dmsvc := newTestDataManager(t, "arm1", "")
//	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//
//	// Disable capture on the collector level.
//	err = dmsvc.Update(context.TODO(), testCfg)
//	test.That(t, err, test.ShouldBeNil)
//
//	// Change something else, but the previous collector capture is still disabled.
//	dmCfg.ScheduledSyncDisabled = true
//	err = dmsvc.Update(context.TODO(), testCfg)
//	test.That(t, err, test.ShouldBeNil)
//
//	// We set sync_interval_mins to be about 250ms in the config, so wait 400ms.
//	time.Sleep(time.Millisecond * 400)
//	err = dmsvc.Close(context.TODO())
//	test.That(t, err, test.ShouldBeNil)
//
//	// Test that nothing was captured or synced.
//	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)
//	filesInArmDir, err := readDir(t, armDir)
//	test.That(t, err, test.ShouldNotBeNil)
//	test.That(t, len(filesInArmDir), test.ShouldEqual, 0)
//}
//
//func TestCollectorDisabledThenEnabled(t *testing.T) {
//	// Register mock datasync service with a mock server.
//	rpcServer, mockService := buildAndStartLocalSyncServer(t)
//	defer func() {
//		err := rpcServer.Stop()
//		test.That(t, err, test.ShouldBeNil)
//		resetFolder(t, captureDir)
//		resetFolder(t, armDir)
//	}()
//	defer resetFolder(t, captureDir)
//	defer resetFolder(t, armDir)
//
//	disabledCollectorConfigPath := "services/datamanager/data/fake_robot_with_disabled_collector.json"
//	testCfg := setupConfig(t, disabledCollectorConfigPath)
//	dmCfg, err := getDataManagerConfig(testCfg)
//	test.That(t, err, test.ShouldBeNil)
//	dmCfg.ScheduledSyncDisabled = true
//
//	// Initialize the data manager and update it with our config.
//	dmsvc := newTestDataManager(t, "arm1", "")
//	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//
//	// Disable capture on the collector level.
//	err = dmsvc.Update(context.TODO(), testCfg)
//	test.That(t, err, test.ShouldBeNil)
//
//	// Re-enable capture on the collector level.
//	testCfg = setupConfig(t, configPath)
//	dmCfg, err = getDataManagerConfig(testCfg)
//	test.That(t, err, test.ShouldBeNil)
//	dmCfg.ScheduledSyncDisabled = true
//	err = dmsvc.Update(context.TODO(), testCfg)
//	test.That(t, err, test.ShouldBeNil)
//
//	// We set sync_interval_mins to be about 250ms in the config, so wait 400ms.
//	time.Sleep(time.Millisecond * 400)
//	err = dmsvc.Close(context.TODO())
//	test.That(t, err, test.ShouldBeNil)
//
//	// Test that something was captured but not synced.
//	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)
//	filesInArmDir, err := readDir(t, armDir)
//	test.That(t, err, test.ShouldBeNil)
//	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
//}

// TODO: replace with a case in arbitrary file sunc tests
//// Validates that if the robot config file specifies a directory path in additionalSyncPaths that does not exist,
//// that directory is created (and can be synced on subsequent iterations of syncing).
//func TestCreatesAdditionalSyncPaths(t *testing.T) {
//	td := "additional_sync_path_dir"
//	// Once testing is complete, remove contents from data capture dirs.
//	defer resetFolder(t, captureDir)
//	defer resetFolder(t, armDir)
//	defer resetFolder(t, td)
//
//	// Register mock datasync service with a mock server.
//	rpcServer, _ := buildAndStartLocalSyncServer(t)
//	defer func() {
//		err := rpcServer.Stop()
//		test.That(t, err, test.ShouldBeNil)
//	}()
//
//	testCfg := setupConfig(t, configPath)
//	dmCfg, err := getDataManagerConfig(testCfg)
//	test.That(t, err, test.ShouldBeNil)
//	dmCfg.SyncIntervalMins = syncIntervalMins
//	dmCfg.AdditionalSyncPaths = []string{td}
//
//	// Initialize the data manager and update it with our config. The call to Update(ctx, conf) should create the
//	// arbitrary sync paths directory it in the file system.
//	dmsvc := newTestDataManager(t, "arm1", "")
//	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//	dmsvc.SetWaitAfterLastModifiedSecs(0)
//	err = dmsvc.Update(context.TODO(), testCfg)
//	test.That(t, err, test.ShouldBeNil)
//
//	// Validate the "additional_sync_path_dir" was created. Wait some time to ensure it would have been created.
//	time.Sleep(syncWaitTime)
//	_ = dmsvc.Close(context.TODO())
//	_, err = os.Stat(td)
//	test.That(t, errors.Is(err, nil), test.ShouldBeTrue)
//}

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

func TestGetDurationFromHz(t *testing.T) {
	test.That(t, GetDurationFromHz(0.1), test.ShouldEqual, time.Second*10)
	test.That(t, GetDurationFromHz(0.5), test.ShouldEqual, time.Second*2)
	test.That(t, GetDurationFromHz(1), test.ShouldEqual, time.Second)
	test.That(t, GetDurationFromHz(1000), test.ShouldEqual, time.Millisecond)
	test.That(t, GetDurationFromHz(0), test.ShouldEqual, 0)
}

// TODO: It doesn't look like this is testing anything not tested by other tests? Can probably remove
//func TestAdditionalParamsInConfig(t *testing.T) {
//	conf := setupConfig(t, "services/datamanager/data/robot_with_cam_capture.json")
//	r := getInjectedRobotWithCamera(t)
//
//	dmCfg := &Config{}
//	cfgService := config.Service{
//		Type:                "data_manager",
//		ConvertedAttributes: dmCfg,
//	}
//	logger := golog.NewTestLogger(t)
//	svc, err := NewBuiltIn(context.Background(), r, cfgService, logger)
//	if err != nil {
//		t.Log(err)
//	}
//
//	dmsvc := svc.(internal.DMService)
//
//	defer resetFolder(t, captureDir)
//
//	err = dmsvc.Update(context.Background(), conf)
//	test.That(t, err, test.ShouldBeNil)
//	time.Sleep(captureWaitTime)
//
//	filesInCamDir, err := readDir(t, captureDir+"/camera/c1/ReadImage")
//	test.That(t, err, test.ShouldBeNil)
//	test.That(t, len(filesInCamDir), test.ShouldEqual, 1)
//	info, err := filesInCamDir[0].Info()
//	test.That(t, err, test.ShouldBeNil)
//	test.That(t, info.Size(), test.ShouldBeGreaterThan, emptyFileBytesSize)
//
//	// Verify that after close is called, the collector is no longer writing.
//	err = dmsvc.Close(context.Background())
//	test.That(t, err, test.ShouldBeNil)
//	err = r.Close(context.Background())
//	test.That(t, err, test.ShouldBeNil)
//}

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

func getLocalServerConn(rpcServer rpc.Server, logger golog.Logger) (rpc.ClientConn, error) {
	return rpc.DialDirectGRPC(
		context.Background(),
		rpcServer.InternalAddr().String(),
		logger,
		rpc.WithInsecure(),
	)
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
	``
	return files
}

package builtin

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	m1 "go.viam.com/api/app/model/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/services/datamanager/model"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	captureWaitTime            = time.Millisecond * 25
	syncWaitTime               = time.Millisecond * 100
	testDataManagerServiceName = "DataManager1"
)

var (
	// Robot config which specifies data manager service.
	configPath = "services/datamanager/data/fake_robot_with_data_manager.json"

	// 0.0041 mins is 246 milliseconds, this is the interval waiting time in the config file used for testing.
	configSyncIntervalMins = 0.0041

	syncIntervalMins   = 0.0041 // 250ms
	captureDir         = "/tmp/capture"
	armDir = captureDir + "/" + arm.Subtype.String() + "/arm1/EndPosition"
	cameraDir = captureDir+ "/" + camera.Subtype.String() + "/c1/ReadImage"
	localArmDir = captureDir + "/" + arm.Subtype.String() + "/localArm/EndPosition"
	remoteArmDir = captureDir + "/" + arm.Subtype.String() + "/remoteArm/EndPosition"

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
	injectedArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
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

func TestNewDataManager(t *testing.T) {
	dmsvc := newTestDataManager(t, "arm1", "")
	testCfg := setupConfig(t, configPath)

	// Empty config at initialization.
	captureDir := "/tmp/capture"
	defer resetFolder(t, captureDir)
	resetFolder(t, captureDir)
	err := dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)
	captureTime := time.Millisecond * 100
	time.Sleep(captureTime)

	err = dmsvc.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Check that a collector wrote to file.
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
	f, err := datacapture.ReadFile(file)
	test.That(t, err, test.ShouldBeNil)
	md := f.ReadMetadata()
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

func TestNewRemoteDataManager(t *testing.T) {
	// Empty config at initialization.
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
	filesInLocalArmDir, err := readDir(t, localArmDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInLocalArmDir), test.ShouldEqual, 1)
	info, err := filesInLocalArmDir[0].Info()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, info.Size(), test.ShouldBeGreaterThan, 0)

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

func TestCollectorDisabled(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
		resetFolder(t, captureDir)
		resetFolder(t, armDir)
	}()
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)

	disabledCollectorConfigPath := "services/datamanager/data/fake_robot_with_disabled_collector.json"
	testCfg := setupConfig(t, disabledCollectorConfigPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))

	// Disable capture on the collector level.
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Change something else, but the previous collector capture is still disabled.
	dmCfg.ScheduledSyncDisabled = true
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// We set sync_interval_mins to be about 250ms in the config, so wait 400ms.
	time.Sleep(time.Millisecond * 400)
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)

	// Test that nothing was captured or synced.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)
	filesInArmDir, err := readDir(t, armDir)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, len(filesInArmDir), test.ShouldEqual, 0)
}

func TestCollectorDisabledThenEnabled(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
		resetFolder(t, captureDir)
		resetFolder(t, armDir)
	}()
	defer resetFolder(t, captureDir)
	defer resetFolder(t, armDir)

	disabledCollectorConfigPath := "services/datamanager/data/fake_robot_with_disabled_collector.json"
	testCfg := setupConfig(t, disabledCollectorConfigPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.ScheduledSyncDisabled = true

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))

	// Disable capture on the collector level.
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Re-enable capture on the collector level.
	testCfg = setupConfig(t, configPath)
	dmCfg, err = getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.ScheduledSyncDisabled = true
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// We set sync_interval_mins to be about 250ms in the config, so wait 400ms.
	time.Sleep(time.Millisecond * 400)
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)

	// Test that something was captured but not synced.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)
	filesInArmDir, err := readDir(t, armDir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
}

// TODO(DATA-341): Handle partial downloads in order to resume deployment.
// TODO(DATA-344): Compare checksum of downloaded model to blob to determine whether to redeploy.
// TODO(DATA-493): Test model deployment from config file.
// TODO(DATA-510): Make TestModelDeploy concurrency safe
// Validates that models can be deployed onto a robot.
func TestModelDeploy(t *testing.T) {
	t.Skip()
	deployModelWaitTime := time.Millisecond * 1000
	deployedZipFileName := "model.zip"
	originalFileName := "model.txt"
	otherOriginalFileName := "README.md"
	b0 := []byte("text representing model.txt internals.")
	b1 := []byte("text representing README.md internals.")

	// Create zip file.
	deployedZipFile, err := os.Create(deployedZipFileName)
	test.That(t, err, test.ShouldBeNil)
	zipWriter := zip.NewWriter(deployedZipFile)

	defer os.Remove(deployedZipFileName)
	defer deployedZipFile.Close()

	// Write zip file contents
	zipFile1, err := zipWriter.Create(originalFileName)
	test.That(t, err, test.ShouldBeNil)
	_, err = zipFile1.Write(b0)
	test.That(t, err, test.ShouldBeNil)

	zipFile2, err := zipWriter.Create(otherOriginalFileName)
	test.That(t, err, test.ShouldBeNil)
	_, err = zipFile2.Write(b1)
	test.That(t, err, test.ShouldBeNil)

	// Close zipWriter so we can unzip later
	zipWriter.Close()

	// Register mock model service with a mock server.
	modelServer, _ := buildAndStartLocalModelServer(t, deployedZipFileName)
	defer func() {
		err := modelServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	// Generate models.
	var allModels []*model.Model
	m1 := &model.Model{Name: "m1", Destination: filepath.Join(os.Getenv("HOME"), "custom")} // with custom location
	m2 := &model.Model{Name: "m2", Destination: ""}                                         // with default location
	allModels = append(allModels, m1, m2)

	defer func() {
		for i := range allModels {
			resetFolder(t, allModels[i].Destination)
		}
	}()

	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Set SyncIntervalMins equal to zero so we do not enable syncing.
	dmCfg.SyncIntervalMins = 0
	dmCfg.ModelsToDeploy = allModels

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetModelManagerConstructor(getTestModelManagerConstructor(t, modelServer, deployedZipFileName))

	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	time.Sleep(deployModelWaitTime)

	// Close the data manager.
	_ = dmsvc.Close(context.Background())

	// Validate that the models were deployed.
	files, err := ioutil.ReadDir(allModels[0].Destination)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 2)

	files, err = ioutil.ReadDir(allModels[1].Destination)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 2)

	// Validate that the deployed model files equal the dummy files that were zipped.
	similar, err := fileCompareTestHelper(filepath.Join(allModels[0].Destination, originalFileName), b0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)

	similar, err = fileCompareTestHelper(filepath.Join(allModels[0].Destination, otherOriginalFileName), b1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)

	similar, err = fileCompareTestHelper(filepath.Join(allModels[1].Destination, originalFileName), b0)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)

	similar, err = fileCompareTestHelper(filepath.Join(allModels[1].Destination, otherOriginalFileName), b1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, similar, test.ShouldBeTrue)
}

func fileCompareTestHelper(path string, info []byte) (bool, error) {
	deployedUnzippedFile, err := ioutil.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Equal(deployedUnzippedFile, info), nil
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
	dmCfg.SyncIntervalMins = syncIntervalMins
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
	dmCfg.SyncIntervalMins = configSyncIntervalMins
	dmCfg.AdditionalSyncPaths = dirs

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Run and upload files.
	err = dmsvc.Sync(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(syncWaitTime)

	// Verify that one data capture file was uploaded, two additional_sync_paths files were uploaded,
	// and that no two uploaded files are the same.
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+1)
	test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)

	// Sync again and verify it synced the second data capture file, but also validate that it didn't attempt to resync
	// any files that were previously synced.
	err = dmsvc.Sync(context.Background(), map[string]interface{}{})
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
	dmCfg.SyncIntervalMins = configSyncIntervalMins
	dmCfg.AdditionalSyncPaths = dirs

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	armDir := captureDir + "/arm/arm1/EndPosition"

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
	dmCfg.SyncIntervalMins = configSyncIntervalMins
	dmCfg.AdditionalSyncPaths = dirs

	// Make the captureDir where we're logging data for our arm.
	defer resetFolder(t, armDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	dmsvc.SetWaitAfterLastModifiedSecs(0)
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Perform a manual and scheduled syncDataCaptureFiles at approximately the same time, then close the svc.
	time.Sleep(time.Millisecond * 250)
	err = dmsvc.Sync(context.TODO(), map[string]interface{}{})
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

func TestSyncEnabledThenDisabled(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.SyncIntervalMins = syncIntervalMins

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

func TestSyncAlwaysDisabled(t *testing.T) {
	// Register mock datasync service with a mock server.
	rpcServer, mockService := buildAndStartLocalServer(t)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	testCfg := setupConfig(t, configPath)
	dmCfg, err := getDataManagerConfig(testCfg)
	test.That(t, err, test.ShouldBeNil)
	dmCfg.ScheduledSyncDisabled = true
	dmCfg.SyncIntervalMins = syncIntervalMins

	// Make the captureDir where we're logging data for our arm.
	captureDir := "/tmp/capture"
	resetFolder(t, captureDir)
	defer resetFolder(t, captureDir)

	// Initialize the data manager and update it with our config.
	dmsvc := newTestDataManager(t, "arm1", "")
	dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
	err = dmsvc.Update(context.TODO(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// We set sync_interval_mins to be about 250ms in the config, so wait 300ms.
	time.Sleep(time.Millisecond * 300)

	// Simulate adding an additional sync path, which would error on Update if we were
	// actually trying to sync.
	dmCfg.AdditionalSyncPaths = []string{"doesnt matter"}
	err = dmsvc.Update(context.Background(), testCfg)
	test.That(t, err, test.ShouldBeNil)

	// Wait and ensure nothing was synced.
	time.Sleep(time.Millisecond * 600)
	err = dmsvc.Close(context.TODO())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)
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

	filesInCamDir, err := readDir(t, cameraDir)
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

// TODO(DATA-487): Support zipping multiple files in
// buildAndStartLocalModelServer and getTestModelManagerConstructor
type mockModelServiceServer struct {
	zipFileName string
	lock        *sync.Mutex
	m1.UnimplementedModelServiceServer
}

func (m mockDataSyncServiceServer) getUploadedFiles() []string {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	return *m.uploadedFiles
}

type mockClient struct {
	zipFileName string
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	buf, err := os.ReadFile(m.zipFileName)
	if err != nil {
		return nil, err
	}

	// convert bytes into bytes.Reader
	reader := bytes.NewReader(buf)

	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(reader),
	}

	return response, nil
}

func (m mockModelServiceServer) Deploy(ctx context.Context, req *m1.DeployRequest) (*m1.DeployResponse, error) {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	depResp := &m1.DeployResponse{Message: m.zipFileName}
	return depResp, nil
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

//nolint:thelper
func buildAndStartLocalModelServer(t *testing.T, deployedZipFileName string) (rpc.Server, mockModelServiceServer) {
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	mockService := mockModelServiceServer{
		zipFileName:                     deployedZipFileName,
		lock:                            &sync.Mutex{},
		UnimplementedModelServiceServer: m1.UnimplementedModelServiceServer{},
	}
	err = rpcServer.RegisterServiceServer(
		context.Background(),
		&m1.ModelService_ServiceDesc,
		mockService,
		m1.RegisterModelServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)

	// Stand up the server. Defer stopping the server.
	go func() {
		err := rpcServer.Start()
		test.That(t, err, test.ShouldBeNil)
	}()
	return rpcServer, mockService
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

//nolint:thelper
func getTestModelManagerConstructor(t *testing.T, server rpc.Server, zipFileName string) model.ManagerConstructor {
	return func(logger golog.Logger, cfg *config.Config) (model.Manager, error) {
		conn, err := getLocalServerConn(server, logger)
		test.That(t, err, test.ShouldBeNil)
		client := model.NewClient(conn)
		return model.NewManager(logger, cfg.Cloud.ID, client, conn, &mockClient{zipFileName: zipFileName})
	}
}

func TestDataCapture(t *testing.T) {
	tests := []struct {
		name                  string
		initialDisabledStatus bool
		newDisabledStatus     bool
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
			defer os.RemoveAll(tmpDir)
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

			svcConfig.CaptureDisabled = tc.initialDisabledStatus
			svcConfig.ScheduledSyncDisabled = true
			svcConfig.CaptureDir = tmpDir
			err = dmsvc.Update(context.Background(), testCfg)
			test.That(t, err, test.ShouldBeNil)

			// let run for a moment
			time.Sleep(captureWaitTime)

			// Check if data has been captured (or not) as we'd expect.
			initialCaptureFiles := getAllFiles(tmpDir)
			initialCaptureFilesSize := getTotalFileSize(initialCaptureFiles)

			if !tc.initialDisabledStatus {
				test.That(t, len(initialCaptureFiles), test.ShouldBeGreaterThan, 0)
				test.That(t, initialCaptureFilesSize, test.ShouldBeGreaterThan, 0)
			} else {
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
				test.That(t, initialCaptureFilesSize, test.ShouldEqual, 0)
			}

			// change status
			svcConfig.CaptureDisabled = tc.newDisabledStatus
			err = dmsvc.Update(context.Background(), testCfg)
			test.That(t, err, test.ShouldBeNil)

			// let run for a moment
			time.Sleep(captureWaitTime)
			midCaptureFiles := getAllFiles(tmpDir)
			midCaptureFilesSize := getTotalFileSize(midCaptureFiles)

			time.Sleep(captureWaitTime)

			updatedCaptureFiles := getAllFiles(tmpDir)
			updatedCaptureFilesSize := getTotalFileSize(updatedCaptureFiles)
			if tc.initialDisabledStatus && !tc.newDisabledStatus {
				// capture disabled then enabled
				test.That(t, len(updatedCaptureFiles), test.ShouldBeGreaterThan, len(initialCaptureFiles))
			} else if !tc.initialDisabledStatus && !tc.newDisabledStatus {
				// capture always enabled
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, len(initialCaptureFiles))
			} else {
				// capture starts disabled/enabled and ends disabled
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, len(initialCaptureFiles))
				test.That(t, updatedCaptureFilesSize, test.ShouldEqual, midCaptureFilesSize)
			}
			test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
		})
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

func getTotalFileSize(files []os.FileInfo) int64 {
	var totalFileSize int64 = 0
	for _, f := range files {
		totalFileSize += f.Size()
	}
	return totalFileSize
}

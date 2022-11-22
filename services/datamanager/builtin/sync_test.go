package builtin

import (
	"context"
	"fmt"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSyncEnabled(t *testing.T) {
	// TODO: this needs to be longer than 1 sec because the syncer hits the queue once a second to check if something is ready to sync
	//       we should make that configurable and use a smaller value in tests
	syncTime := time.Millisecond * 100

	tests := []struct {
		name                        string
		initialServiceDisableStatus bool
		newServiceDisableStatus     bool
	}{
		{
			name:                        "Config with sync disabled should sync nothing.",
			initialServiceDisableStatus: true,
			newServiceDisableStatus:     true,
		},
		{
			name:                        "Config with sync enabled should sync.",
			initialServiceDisableStatus: false,
			newServiceDisableStatus:     false,
		},
		{
			name:                        "Disabling sync should stop syncing.",
			initialServiceDisableStatus: false,
			newServiceDisableStatus:     true,
		},
		{
			name:                        "Enabling sync should trigger syncing to start.",
			initialServiceDisableStatus: true,
			newServiceDisableStatus:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// TODO: this is a sign of an abstraction leak. Should probably be a construction parameter,
			//       and then used for a ticker instead of Sleep
			datasync.PollWaitTime = time.Millisecond * 25

			// Set up server.
			tmpDir, err := os.MkdirTemp("", "")
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				err := os.RemoveAll(tmpDir)
				test.That(t, err, test.ShouldBeNil)
			}()
			rpcServer, mockService := buildAndStartLocalSyncServer(t)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
			cfg := setupConfig(t, enabledTabularCollectorConfigPath)

			// Set up service config.
			originalSvcConfig, ok1, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			originalSvcConfig.CaptureDisabled = false
			originalSvcConfig.ScheduledSyncDisabled = tc.initialServiceDisableStatus
			originalSvcConfig.CaptureDir = tmpDir
			originalSvcConfig.SyncIntervalMins = 0.1

			err = dmsvc.Update(context.Background(), cfg)

			// Let run for a second, then change status.
			time.Sleep(syncTime)

			// Things to validate: that it syncs if expected, that it deletes files if successful
			initialUploadCount := len(mockService.getCaptureUploadRequests())
			if !tc.initialServiceDisableStatus {
				// TODO: check contents
				test.That(t, initialUploadCount, test.ShouldBeGreaterThan, 0)
			} else {
				test.That(t, initialUploadCount, test.ShouldEqual, 0)
			}

			// Set up service config.
			updatedSvcConfig, ok2, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok2, test.ShouldBeTrue)
			updatedSvcConfig.CaptureDisabled = false
			updatedSvcConfig.ScheduledSyncDisabled = tc.newServiceDisableStatus
			updatedSvcConfig.CaptureDir = tmpDir
			updatedSvcConfig.SyncIntervalMins = 0.016

			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// Let run for a second, then change status.
			time.Sleep(syncTime)
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			newUploadCount := len(mockService.getCaptureUploadRequests())
			// TODO: Things to validate: that it syncs if expected, that it deletes files if successful
			if !tc.newServiceDisableStatus {
				test.That(t, newUploadCount, test.ShouldBeGreaterThan, initialUploadCount)
			} else {
				test.That(t, newUploadCount, test.ShouldEqual, initialUploadCount)
			}
		})
	}
}

/*
*
TEST SETUP:
- Let capture run for a bit.
- Kill it.
- Build new SUT.
- Let sync run (probably with capture disabled?)
- Ensure all readings captured were synced.
- Ensure no files remain

- Variations? Maybe one for binary and one for tabular
*/
func TestResumableUpload(t *testing.T) {
	tests := []struct {
		name          string
		dataType      v1.DataType
		serviceFailAt int32
	}{
		{
			name:     "Previously captured tabular data should be synced at start up.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		},
		{
			name:     "Previously captured binary data should be synced at start up.",
			dataType: v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name:          "If tabular sync fails part way through, it should be resumed without duplicate uploads",
			dataType:      v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			serviceFailAt: 2,
		},
		{
			name:          "If binary sync fails part way through, it should be resumed without duplicate uploads",
			dataType:      v1.DataType_DATA_TYPE_BINARY_SENSOR,
			serviceFailAt: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			datasync.PollWaitTime = time.Millisecond * 25

			// Set up server.
			tmpDir, err := os.MkdirTemp("", "")
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				err := os.RemoveAll(tmpDir)
				test.That(t, err, test.ShouldBeNil)
			}()
			rpcServer, mockService := buildAndStartLocalSyncServer(t)
			datacapture.MaxFileSize = 150
			// MaxFileSize of 150 => Should be 2 tabular readings per file/UR
			sensorDataPerUploadRequest := 2.0
			failAt := &atomic.Int32{}
			failAt.Add(tc.serviceFailAt)
			mockService.failAt = failAt
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
			var cfg *config.Config
			if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
				cfg = setupConfig(t, enabledTabularCollectorConfigPath)
			} else {
				cfg = setupConfig(t, enabledBinaryCollectorConfigPath)
			}

			// Set up service config.
			originalSvcConfig, ok1, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			originalSvcConfig.CaptureDisabled = false
			originalSvcConfig.ScheduledSyncDisabled = true
			originalSvcConfig.SyncIntervalMins = 0.016
			originalSvcConfig.CaptureDir = tmpDir

			err = dmsvc.Update(context.Background(), cfg)
			fmt.Println("successfully updated to capture")
			test.That(t, err, test.ShouldBeNil)

			// Let run for a second, then close.
			time.Sleep(time.Millisecond * 100)
			err = dmsvc.Close(context.Background())
			fmt.Println("successfully closed capture")
			test.That(t, err, test.ShouldBeNil)

			// Get all written data.
			capturedData, err := getCapturedData(tmpDir)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(capturedData), test.ShouldBeGreaterThan, 0)

			// Now turn back on with only sync enabled.
			newDMSvc := newTestDataManager(t)
			newDMSvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
			originalSvcConfig.CaptureDisabled = true
			originalSvcConfig.ScheduledSyncDisabled = false
			originalSvcConfig.SyncIntervalMins = 0.016
			err = newDMSvc.Update(context.Background(), cfg)
			fmt.Println("successfully updated to sync")
			test.That(t, err, test.ShouldBeNil)
			time.Sleep(time.Millisecond * 100)
			err = newDMSvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)
			fmt.Println("successfully closed")

			urs := mockService.getCaptureUploadRequests()
			if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
				test.That(t, len(urs), test.ShouldEqual, math.Ceil(float64(len(capturedData))/sensorDataPerUploadRequest))
			} else {
				test.That(t, len(urs), test.ShouldEqual, len(capturedData))
			}

			syncedData := getUploadedData(urs)
			compareSensorData(t, tc.dataType, syncedData, capturedData)

			// After all uploads succeed, their files should be deleted.
			test.That(t, len(getAllFiles(tmpDir)), test.ShouldEqual, 0)
		})
	}
}

// TODO: ensure that when syncs fail, files are not deleted. Ensure that when syncs fail transiently, they get retried.
//
//	succeed. Ensure that when things repeatedly fail, Close is still respected.
func TestRetriesUploads(t *testing.T) {
	tests := []struct {
		name     string
		dataType v1.DataType
		numFails int32
	}{
		{
			name:     "If transient errors occur during tabular upload, they should be retried until they succeed.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			numFails: 2,
		},
		{
			name:     "If transient errors occur during binary upload, they should be retried until they succeed.",
			dataType: v1.DataType_DATA_TYPE_BINARY_SENSOR,
			numFails: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

		})
	}
}

func getCapturedData(dir string) ([]*v1.SensorData, error) {
	var allFiles []*datacapture.File
	var filePaths []string

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		filePaths = append(filePaths, path)
		return nil
	})

	for _, f := range filePaths {
		osFile, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		dcFile, err := datacapture.ReadFile(osFile)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, dcFile)
	}

	var ret []*v1.SensorData
	for _, dcFile := range allFiles {
		for {
			next, err := dcFile.ReadNext()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			ret = append(ret, next)
		}
	}
	return ret, nil
}

func getUploadedData(urs []*v1.DataCaptureUploadRequest) []*v1.SensorData {
	var syncedData []*v1.SensorData
	for _, ur := range urs {
		sd := ur.GetSensorContents()
		syncedData = append(syncedData, sd...)
	}
	return syncedData
}

func compareSensorData(t *testing.T, dataType v1.DataType, act []*v1.SensorData, exp []*v1.SensorData) {
	t.Helper()
	if len(act) == 0 && len(exp) == 0 {
		return
	}

	test.That(t, len(act), test.ShouldEqual, len(exp))
	if dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
		for i := range act {
			test.That(t, act[i].GetStruct(), test.ShouldResemble, exp[i].GetStruct())
			test.That(t, act[i].GetMetadata(), test.ShouldResemble, exp[i].GetMetadata())
		}
	} else {
		for i := range act {
			test.That(t, act[i].GetBinary(), test.ShouldResemble, exp[i].GetBinary())
			test.That(t, act[i].GetMetadata(), test.ShouldResemble, exp[i].GetMetadata())
		}
	}
}

// TODO: combine manual, "scheduled", manualandscheduled into single table driven test suite
// Validates that manual syncing works for a datamanager.
//
//	func TestManualSync(t *testing.T) {
//		// Register mock datasync service with a mock server.
//		rpcServer, mockService := buildAndStartLocalSyncServer(t)
//		defer func() {
//			err := rpcServer.Stop()
//			test.That(t, err, test.ShouldBeNil)
//		}()
//
//		dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
//		defer func() {
//			for _, dir := range dirs {
//				resetFolder(t, dir)
//			}
//		}()
//		defer resetFolder(t, captureDir)
//		defer resetFolder(t, armDir)
//		if err != nil {
//			t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
//		}
//		testCfg := setupConfig(t, configPath)
//		dmCfg, err := getDataManagerConfig(testCfg)
//		test.That(t, err, test.ShouldBeNil)
//		dmCfg.SyncIntervalMins = configSyncIntervalMins
//		dmCfg.AdditionalSyncPaths = dirs
//
//		// Initialize the data manager and update it with our config.
//		dmsvc := newTestDataManager(t, "arm1", "")
//		dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//		dmsvc.SetWaitAfterLastModifiedSecs(0)
//		err = dmsvc.Update(context.TODO(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// Run and upload files.
//		err = dmsvc.Sync(context.Background(), map[string]interface{}{})
//		test.That(t, err, test.ShouldBeNil)
//		time.Sleep(syncWaitTime)
//
//		// Verify that one data capture file was uploaded, two additional_sync_paths files were uploaded,
//		// and that no two uploaded files are the same.
//		test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+1)
//		test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)
//
//		// SyncCaptureQueues again and verify it synced the second data capture file, but also validate that it didn't attempt to resync
//		// any files that were previously synced.
//		err = dmsvc.Sync(context.Background(), map[string]interface{}{})
//		test.That(t, err, test.ShouldBeNil)
//		time.Sleep(syncWaitTime)
//		_ = dmsvc.Close(context.TODO())
//		test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+2)
//		test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)
//	}
//
// // Validates that scheduled syncing works for a datamanager.
//
//	func TestScheduledSync(t *testing.T) {
//		// Register mock datasync service with a mock server.
//		rpcServer, mockService := buildAndStartLocalSyncServer(t)
//		defer func() {
//			err := rpcServer.Stop()
//			test.That(t, err, test.ShouldBeNil)
//		}()
//
//		dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
//		defer func() {
//			for _, dir := range dirs {
//				_ = os.RemoveAll(dir)
//			}
//		}()
//		defer resetFolder(t, captureDir)
//		defer resetFolder(t, armDir)
//		if err != nil {
//			t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
//		}
//		// Use config with 250ms sync interval.
//		testCfg := setupConfig(t, configPath)
//		dmCfg, err := getDataManagerConfig(testCfg)
//		test.That(t, err, test.ShouldBeNil)
//		dmCfg.SyncIntervalMins = configSyncIntervalMins
//		dmCfg.AdditionalSyncPaths = dirs
//
//		// Make the captureDir where we're logging data for our arm.
//		captureDir := "/tmp/capture"
//		armDir := captureDir + "/arm/arm1/EndPosition"
//
//		// Clear the capture dir after we're done.
//		defer resetFolder(t, armDir)
//
//		// Initialize the data manager and update it with our config.
//		dmsvc := newTestDataManager(t, "arm1", "")
//		dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//		dmsvc.SetWaitAfterLastModifiedSecs(0)
//		err = dmsvc.Update(context.TODO(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// We set sync_interval_mins to be about 250ms in the config, so wait 600ms (more than two iterations of syncing)
//		// for the additional_sync_paths files to sync AND for TWO data capture files to sync.
//		time.Sleep(time.Millisecond * 600)
//		_ = dmsvc.Close(context.TODO())
//
//		// Verify that the additional_sync_paths files AND the TWO data capture files were uploaded.
//		test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+2)
//		test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)
//	}
//
// // Validates that we can attempt a scheduled and manual syncDataCaptureFiles at the same time without duplicating files
// // or running into errors.
//
//	func TestManualAndScheduledSync(t *testing.T) {
//		// Register mock datasync service with a mock server.
//		rpcServer, mockService := buildAndStartLocalSyncServer(t)
//		defer func() {
//			err := rpcServer.Stop()
//			test.That(t, err, test.ShouldBeNil)
//		}()
//
//		dirs, numArbitraryFilesToSync, err := populateAdditionalSyncPaths()
//		defer func() {
//			for _, dir := range dirs {
//				resetFolder(t, dir)
//			}
//		}()
//		defer resetFolder(t, captureDir)
//		defer resetFolder(t, armDir)
//		if err != nil {
//			t.Error("unable to generate arbitrary data files and create directory structure for additionalSyncPaths")
//		}
//		testCfg := setupConfig(t, configPath)
//		dmCfg, err := getDataManagerConfig(testCfg)
//		test.That(t, err, test.ShouldBeNil)
//		dmCfg.SyncIntervalMins = configSyncIntervalMins
//		dmCfg.AdditionalSyncPaths = dirs
//
//		// Make the captureDir where we're logging data for our arm.
//		captureDir := "/tmp/capture"
//		armDir := captureDir + "/arm/arm1/EndPosition"
//		defer resetFolder(t, armDir)
//
//		// Initialize the data manager and update it with our config.
//		dmsvc := newTestDataManager(t, "arm1", "")
//		dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//		dmsvc.SetWaitAfterLastModifiedSecs(0)
//		err = dmsvc.Update(context.TODO(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// Perform a manual and scheduled syncDataCaptureFiles at approximately the same time, then close the svc.
//		time.Sleep(time.Millisecond * 250)
//		err = dmsvc.Sync(context.TODO(), map[string]interface{}{})
//		test.That(t, err, test.ShouldBeNil)
//		time.Sleep(syncWaitTime)
//		_ = dmsvc.Close(context.TODO())
//
//		// Verify that two data capture files were uploaded, two additional_sync_paths files were uploaded,
//		// and that no two uploaded files are the same.
//		test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, numArbitraryFilesToSync+2)
//		test.That(t, noRepeatedElements(mockService.getUploadedFiles()), test.ShouldBeTrue)
//
//		// We've uploaded (and thus deleted) the first two files and should now be collecting a single new one.
//		filesInArmDir, err := readDir(t, armDir)
//		if err != nil {
//			t.Fatalf("failed to list files in armDir")
//		}
//		test.That(t, len(filesInArmDir), test.ShouldEqual, 1)
//	}
//
// // TODO: replace following two tests with single table driven test suite similar to TestDataCaptureEnabled
//
//	func TestSyncEnabledThenDisabled(t *testing.T) {
//		// Register mock datasync service with a mock server.
//		rpcServer, mockService := buildAndStartLocalSyncServer(t)
//		defer func() {
//			err := rpcServer.Stop()
//			test.That(t, err, test.ShouldBeNil)
//		}()
//
//		testCfg := setupConfig(t, configPath)
//		dmCfg, err := getDataManagerConfig(testCfg)
//		test.That(t, err, test.ShouldBeNil)
//		dmCfg.SyncIntervalMins = syncIntervalMins
//
//		// Make the captureDir where we're logging data for our arm.
//		captureDir := "/tmp/capture"
//		resetFolder(t, captureDir)
//		defer resetFolder(t, captureDir)
//
//		// Initialize the data manager and update it with our config.
//		dmsvc := newTestDataManager(t, "arm1", "")
//		dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//		err = dmsvc.Update(context.TODO(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// We set sync_interval_mins to be about 250ms in the config, so wait 150ms so data is captured but not synced.
//		time.Sleep(time.Millisecond * 150)
//
//		// Simulate disabling sync.
//		dmCfg.ScheduledSyncDisabled = true
//		err = dmsvc.Update(context.Background(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// Validate nothing has been synced yet.
//		test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)
//
//		// Re-enable sync.
//		dmCfg.ScheduledSyncDisabled = false
//		err = dmsvc.Update(context.Background(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// We set sync_interval_mins to be about 250ms in the config, so wait 600ms and ensure three files were uploaded:
//		// one from file immediately uploaded when sync was re-enabled and two after.
//		time.Sleep(time.Millisecond * 600)
//		err = dmsvc.Close(context.TODO())
//		test.That(t, err, test.ShouldBeNil)
//		test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 3)
//	}
//
//	func TestSyncAlwaysDisabled(t *testing.T) {
//		// Register mock datasync service with a mock server.
//		rpcServer, mockService := buildAndStartLocalSyncServer(t)
//		defer func() {
//			err := rpcServer.Stop()
//			test.That(t, err, test.ShouldBeNil)
//		}()
//
//		testCfg := setupConfig(t, configPath)
//		dmCfg, err := getDataManagerConfig(testCfg)
//		test.That(t, err, test.ShouldBeNil)
//		dmCfg.ScheduledSyncDisabled = true
//		dmCfg.SyncIntervalMins = syncIntervalMins
//
//		// Make the captureDir where we're logging data for our arm.
//		captureDir := "/tmp/capture"
//		resetFolder(t, captureDir)
//		defer resetFolder(t, captureDir)
//
//		// Initialize the data manager and update it with our config.
//		dmsvc := newTestDataManager(t, "arm1", "")
//		dmsvc.SetSyncerConstructor(getTestSyncerConstructor(t, rpcServer))
//		err = dmsvc.Update(context.TODO(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// We set sync_interval_mins to be about 250ms in the config, so wait 300ms.
//		time.Sleep(time.Millisecond * 300)
//
//		// Simulate adding an additional sync path, which would error on Update if we were
//		// actually trying to sync.
//		dmCfg.AdditionalSyncPaths = []string{"doesnt matter"}
//		err = dmsvc.Update(context.Background(), testCfg)
//		test.That(t, err, test.ShouldBeNil)
//
//		// Wait and ensure nothing was synced.
//		time.Sleep(time.Millisecond * 600)
//		err = dmsvc.Close(context.TODO())
//		test.That(t, err, test.ShouldBeNil)
//		test.That(t, len(mockService.getUploadedFiles()), test.ShouldEqual, 0)
//	}
//
// // Generates and populates a directory structure of files that contain arbitrary file data. Used to simulate testing
// // syncing of data in the service's additional_sync_paths.
// // nolint
//
//	func populateAdditionalSyncPaths() ([]string, int, error) {
//		var additionalSyncPaths []string
//		numArbitraryFilesToSync := 0
//
//		// Generate additional_sync_paths "dummy" dirs & files.
//		for i := 0; i < 2; i++ {
//			// Create a temp dir that will be in additional_sync_paths.
//			td, err := os.MkdirTemp("", "additional_sync_path_dir_")
//			if err != nil {
//				return []string{}, 0, errors.New("cannot create temporary dir to simulate additional_sync_paths in data manager service config")
//			}
//			additionalSyncPaths = append(additionalSyncPaths, td)
//
//			// Make the first dir empty.
//			if i == 0 {
//				continue
//			} else {
//				// Make the dirs that will contain two file.
//				for i := 0; i < 2; i++ {
//					// Generate data that will be in a temp file.
//					fileData := []byte("This is file data. It will be stored in a directory included in the user's specified additional sync paths. Hopefully it is uploaded from the robot to the cloud!")
//
//					// Create arbitrary file that will be in the temp dir generated above.
//					tf, err := os.CreateTemp(td, "arbitrary_file_")
//					if err != nil {
//						return nil, 0, errors.New("cannot create temporary file to simulate uploading from data manager service")
//					}
//
//					// Write data to the temp file.
//					if _, err := tf.Write(fileData); err != nil {
//						return nil, 0, errors.New("cannot write arbitrary data to temporary file")
//					}
//
//					// Increment number of files to be synced.
//					numArbitraryFilesToSync++
//				}
//			}
//		}
//		return additionalSyncPaths, numArbitraryFilesToSync, nil
//	}
//
// // TODO: mocks below this point. Maybe reconsider organization.
//
//nolint:thelper
func getTestSyncerConstructor(t *testing.T, server rpc.Server) datasync.ManagerConstructor {
	return func(logger golog.Logger, cfg *config.Config) (datasync.Manager, error) {
		conn, err := getLocalServerConn(server, logger)
		test.That(t, err, test.ShouldBeNil)
		client := datasync.NewClient(conn)
		return datasync.NewManager(logger, cfg.Cloud.ID, client, conn)
	}
}

type mockDataSyncServiceServer struct {
	dataCaptureUploadRequests *[]*v1.DataCaptureUploadRequest
	fileUploadRequests        *[]*v1.FileUploadRequest
	lock                      *sync.Mutex
	failAt                    *atomic.Int32
	failFor                   *atomic.Int32
	failCount                 *atomic.Int32
	callCount                 *atomic.Int32
	v1.UnimplementedDataSyncServiceServer
}

func (m *mockDataSyncServiceServer) getCaptureUploadRequests() []*v1.DataCaptureUploadRequest {
	m.lock.Lock()
	defer m.lock.Unlock()
	return *m.dataCaptureUploadRequests
}

func (m mockDataSyncServiceServer) DataCaptureUpload(ctx context.Context, ur *v1.DataCaptureUploadRequest) (*v1.DataCaptureUploadResponse, error) {
	defer m.callCount.Add(1)
	(*m.lock).Lock()
	*m.dataCaptureUploadRequests = append(*m.dataCaptureUploadRequests, ur)
	(*m.lock).Unlock()
	if m.failAt.Load() != 0 && m.callCount.Load() >= m.failAt.Load() {
		m.failCount.Add(1)
		return nil, errors.New("oh no error!!")
	}

	if m.failFor.Load() != 0 && m.failCount.Load() >= m.failFor.Load() {
		m.failCount.Add(1)
		return nil, errors.New("oh no error!!")
	}
	// TODO: will likely need to make this optionally return errors for testing error cases
	return &v1.DataCaptureUploadResponse{
		Code:    200,
		Message: "",
	}, nil
}

func (m mockDataSyncServiceServer) FileUpload(stream v1.DataSyncService_FileUploadServer) error {
	return status.Errorf(codes.Unimplemented, "method FileUpload not implemented")
}

//nolint:thelper
func buildAndStartLocalSyncServer(t *testing.T) (rpc.Server, *mockDataSyncServiceServer) {
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	mockService := mockDataSyncServiceServer{
		dataCaptureUploadRequests:          &[]*v1.DataCaptureUploadRequest{},
		lock:                               &sync.Mutex{},
		UnimplementedDataSyncServiceServer: v1.UnimplementedDataSyncServiceServer{},
		failAt:                             &atomic.Int32{},
		callCount:                          &atomic.Int32{},
		failFor:                            &atomic.Int32{},
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

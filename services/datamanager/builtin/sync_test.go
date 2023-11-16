package builtin

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
)

const (
	syncIntervalMins = 0.0008
	syncInterval     = time.Millisecond * 50
)

// TODO DATA-849: Add a test that validates that sync interval is accurately respected.
func TestSyncEnabled(t *testing.T) {
	captureInterval := time.Millisecond * 10
	tests := []struct {
		name                        string
		initialServiceDisableStatus bool
		newServiceDisableStatus     bool
	}{
		{
			name:                        "config with sync disabled should sync nothing",
			initialServiceDisableStatus: true,
			newServiceDisableStatus:     true,
		},
		{
			name:                        "config with sync enabled should sync",
			initialServiceDisableStatus: false,
			newServiceDisableStatus:     false,
		},
		{
			name:                        "disabling sync should stop syncing",
			initialServiceDisableStatus: false,
			newServiceDisableStatus:     true,
		},
		{
			name:                        "enabling sync should trigger syncing to start",
			initialServiceDisableStatus: true,
			newServiceDisableStatus:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			mockClock := clk.NewMock()
			// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
			clock = mockClock
			tmpDir := t.TempDir()

			// Set up data manager.
			dmsvc, r := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			mockClient := mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fail:                &atomic.Bool{},
			}
			dmsvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))
			cfg, deps := setupConfig(t, enabledBinaryCollectorConfigPath)

			// Set up service config.
			cfg.CaptureDisabled = false
			cfg.ScheduledSyncDisabled = tc.initialServiceDisableStatus
			cfg.CaptureDir = tmpDir
			cfg.SyncIntervalMins = syncIntervalMins

			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)
			mockClock.Add(captureInterval)
			waitForCaptureFilesToExceedNFiles(tmpDir, 0)
			mockClock.Add(syncInterval)
			var sentReq bool
			wait := time.After(time.Second)
			select {
			case <-wait:
			case <-mockClient.succesfulDCRequests:
				sentReq = true
			}

			if !tc.initialServiceDisableStatus {
				test.That(t, sentReq, test.ShouldBeTrue)
			} else {
				test.That(t, sentReq, test.ShouldBeFalse)
			}

			// Set up service config.
			cfg.CaptureDisabled = false
			cfg.ScheduledSyncDisabled = tc.newServiceDisableStatus
			cfg.CaptureDir = tmpDir
			cfg.SyncIntervalMins = syncIntervalMins

			resources = resourcesFromDeps(t, r, deps)
			err = dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)

			// Drain any requests that were already sent before Update returned.
			for len(mockClient.succesfulDCRequests) > 0 {
				<-mockClient.succesfulDCRequests
			}
			var sentReqAfterUpdate bool
			mockClock.Add(captureInterval)
			waitForCaptureFilesToExceedNFiles(tmpDir, 0)
			mockClock.Add(syncInterval)
			wait = time.After(time.Second)
			select {
			case <-wait:
			case <-mockClient.succesfulDCRequests:
				sentReqAfterUpdate = true
			}
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			if !tc.newServiceDisableStatus {
				test.That(t, sentReqAfterUpdate, test.ShouldBeTrue)
			} else {
				test.That(t, sentReqAfterUpdate, test.ShouldBeFalse)
			}
		})
	}
}

// TODO DATA-849: Test concurrent capture and sync more thoroughly.
func TestDataCaptureUploadIntegration(t *testing.T) {
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	datasync.RetryExponentialFactor.Store(int32(1))
	datasync.InitialWaitTimeMillis.Store(int32(20))

	tests := []struct {
		name                  string
		dataType              v1.DataType
		manualSync            bool
		scheduledSyncDisabled bool
		failTransiently       bool
		emptyFile             bool
	}{
		{
			name:     "previously captured tabular data should be synced at start up",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		},
		{
			name:     "previously captured binary data should be synced at start up",
			dataType: v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name:                  "manual sync should successfully sync captured tabular data",
			dataType:              v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync:            true,
			scheduledSyncDisabled: true,
		},
		{
			name:                  "manual sync should successfully sync captured binary data",
			dataType:              v1.DataType_DATA_TYPE_BINARY_SENSOR,
			manualSync:            true,
			scheduledSyncDisabled: true,
		},
		{
			name:       "running manual and scheduled sync concurrently should not cause data races or duplicate uploads",
			dataType:   v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync: true,
		},
		{
			name:            "if tabular uploads fail transiently, they should be retried until they succeed",
			dataType:        v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			failTransiently: true,
		},
		{
			name:            "if binary uploads fail transiently, they should be retried until they succeed",
			dataType:        v1.DataType_DATA_TYPE_BINARY_SENSOR,
			failTransiently: true,
		},
		{
			name:      "files with no sensor data should not be synced",
			emptyFile: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			mockClock := clk.NewMock()
			clock = mockClock
			tmpDir := t.TempDir()

			// Set up data manager.
			dmsvc, r := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			var cfg *Config
			var deps []string
			captureInterval := time.Millisecond * 10
			if tc.emptyFile {
				cfg, deps = setupConfig(t, infrequentCaptureTabularCollectorConfigPath)
			} else {
				if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
					cfg, deps = setupConfig(t, enabledTabularCollectorConfigPath)
				} else {
					cfg, deps = setupConfig(t, enabledBinaryCollectorConfigPath)
				}
			}

			// Set up service config with only capture enabled.
			cfg.CaptureDisabled = false
			cfg.ScheduledSyncDisabled = true
			cfg.SyncIntervalMins = syncIntervalMins
			cfg.CaptureDir = tmpDir

			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)

			// Let it capture a bit, then close.
			for i := 0; i < 20; i++ {
				mockClock.Add(captureInterval)
			}
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Get all captured data.
			numFiles, capturedData, err := getCapturedData(tmpDir)
			test.That(t, err, test.ShouldBeNil)
			if tc.emptyFile {
				test.That(t, len(capturedData), test.ShouldEqual, 0)
			} else {
				test.That(t, len(capturedData), test.ShouldBeGreaterThan, 0)
			}

			// Turn dmsvc back on with capture disabled.
			newDMSvc, r := newTestDataManager(t)
			defer newDMSvc.Close(context.Background())
			mockClient := mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fail:                &atomic.Bool{},
			}
			newDMSvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))
			cfg.CaptureDisabled = true
			cfg.ScheduledSyncDisabled = tc.scheduledSyncDisabled
			cfg.SyncIntervalMins = syncIntervalMins
			resources = resourcesFromDeps(t, r, deps)
			err = newDMSvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)

			if tc.failTransiently {
				// Simulate the backend returning errors some number of times, and validate that the dmsvc is continuing
				// to retry.
				numFails := 3
				mockClient.fail.Store(true)
				for i := 0; i < numFails; i++ {
					mockClock.Add(syncInterval)
					// Each time we sync, we should get a sync request for each file.
					for j := 0; j < numFiles; j++ {
						wait := time.After(time.Second * 5)
						select {
						case <-wait:
							t.Fatalf("timed out waiting for sync request")
						case <-mockClient.failedDCRequests:
						}
					}
				}
			}

			mockClient.fail.Store(false)
			// If testing manual sync, call sync. Call it multiple times to ensure concurrent calls are safe.
			if tc.manualSync {
				err = newDMSvc.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
				err = newDMSvc.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
			}

			var successfulReqs []*v1.DataCaptureUploadRequest
			// Get the successful requests
			mockClock.Add(syncInterval)
			for i := 0; i < numFiles; i++ {
				wait := time.After(time.Second * 5)
				select {
				case <-wait:
					t.Fatalf("timed out waiting for sync request")
				case r := <-mockClient.succesfulDCRequests:
					successfulReqs = append(successfulReqs, r)
				}
			}

			// Give it time to delete files after upload.
			waitUntilNoFiles(tmpDir)
			err = newDMSvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Validate that all captured data was synced.
			syncedData := getUploadedData(successfulReqs)
			compareSensorData(t, tc.dataType, syncedData, capturedData)

			// After all uploads succeed, all files should be deleted.
			test.That(t, len(getAllFileInfos(tmpDir)), test.ShouldEqual, 0)
		})
	}
}

func TestArbitraryFileUpload(t *testing.T) {
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	datasync.RetryExponentialFactor.Store(int32(1))
	fileName := "some_file_name.txt"
	fileExt := ".txt"

	tests := []struct {
		name                 string
		manualSync           bool
		scheduleSyncDisabled bool
		serviceFail          bool
	}{
		{
			name:                 "scheduled sync of arbitrary files should work",
			manualSync:           false,
			scheduleSyncDisabled: false,
		},
		{
			name:                 "manual sync of arbitrary files should work",
			manualSync:           true,
			scheduleSyncDisabled: true,
		},
		{
			name:                 "running manual and scheduled sync concurrently should work and not lead to duplicate uploads",
			manualSync:           true,
			scheduleSyncDisabled: false,
		},
		{
			name:                 "if an error response is received from the backend, local files should not be deleted",
			manualSync:           false,
			scheduleSyncDisabled: false,
			serviceFail:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			mockClock := clk.NewMock()
			clock = mockClock
			additionalPathsDir := t.TempDir()
			captureDir := t.TempDir()

			// Set up dmsvc config.
			dmsvc, r := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			f := atomic.Bool{}
			f.Store(tc.serviceFail)
			mockClient := mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fileUploads:         make(chan *mockFileUploadClient, 100),
				fail:                &f,
			}
			dmsvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))
			cfg, deps := setupConfig(t, disabledTabularCollectorConfigPath)
			cfg.ScheduledSyncDisabled = tc.scheduleSyncDisabled
			cfg.SyncIntervalMins = syncIntervalMins
			cfg.AdditionalSyncPaths = []string{additionalPathsDir}
			cfg.CaptureDir = captureDir

			// Start dmsvc.
			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)
			// Ensure that we don't wait to sync files.
			dmsvc.SetFileLastModifiedMillis(0)

			// Write file to the path.
			var fileContents []byte
			for i := 0; i < 1000; i++ {
				fileContents = append(fileContents, []byte("happy cows come from california\n")...)
			}
			tmpFile, err := os.Create(filepath.Join(additionalPathsDir, fileName))
			test.That(t, err, test.ShouldBeNil)
			_, err = tmpFile.Write(fileContents)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, tmpFile.Close(), test.ShouldBeNil)

			// Advance the clock syncInterval so it tries to sync the files.
			mockClock.Add(syncInterval)

			// Call manual sync.
			if tc.manualSync {
				err = dmsvc.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
			}

			// Wait for upload requests.
			var fileUploads []*mockFileUploadClient
			var urs []*v1.FileUploadRequest
			// Get the successful requests
			wait := time.After(time.Second * 3)
			select {
			case <-wait:
				if !tc.serviceFail {
					t.Fatalf("timed out waiting for sync request")
				}
			case r := <-mockClient.fileUploads:
				fileUploads = append(fileUploads, r)
				select {
				case <-wait:
					t.Fatalf("timed out waiting for sync request")
				case <-r.closed:
					urs = append(urs, r.urs...)
				}
			}

			waitUntilNoFiles(additionalPathsDir)
			if !tc.serviceFail {
				// Validate first metadata message.
				test.That(t, len(fileUploads), test.ShouldEqual, 1)
				test.That(t, len(urs), test.ShouldBeGreaterThan, 0)
				actMD := urs[0].GetMetadata()
				test.That(t, actMD, test.ShouldNotBeNil)
				test.That(t, actMD.Type, test.ShouldEqual, v1.DataType_DATA_TYPE_FILE)
				test.That(t, filepath.Base(actMD.FileName), test.ShouldEqual, fileName)
				test.That(t, actMD.FileExtension, test.ShouldEqual, fileExt)
				test.That(t, actMD.PartId, test.ShouldNotBeBlank)

				// Validate ensuing data messages.
				dataRequests := urs[1:]
				var actData []byte
				for _, d := range dataRequests {
					actData = append(actData, d.GetFileContents().GetData()...)
				}
				test.That(t, actData, test.ShouldResemble, fileContents)

				// Validate file no longer exists.
				test.That(t, len(getAllFileInfos(additionalPathsDir)), test.ShouldEqual, 0)
				test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
			} else {
				// Validate no files were successfully uploaded.
				test.That(t, len(fileUploads), test.ShouldEqual, 0)
				// Validate file still exists.
				test.That(t, len(getAllFileInfos(additionalPathsDir)), test.ShouldEqual, 1)
			}
		})
	}
}

func TestStreamingDCUpload(t *testing.T) {
	tests := []struct {
		name        string
		serviceFail bool
	}{
		{
			name: "A data capture file greater than MaxUnaryFileSize should be successfully uploaded" +
				"via the streaming rpc.",
			serviceFail: false,
		},
		{
			name:        "if an error response is received from the backend, local files should not be deleted",
			serviceFail: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			mockClock := clk.NewMock()
			clock = mockClock
			tmpDir := t.TempDir()

			// Set up data manager.
			dmsvc, r := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			var cfg *Config
			var deps []string
			captureInterval := time.Millisecond * 10
			cfg, deps = setupConfig(t, enabledBinaryCollectorConfigPath)

			// Set up service config with just capture enabled.
			cfg.CaptureDisabled = false
			cfg.ScheduledSyncDisabled = true
			cfg.SyncIntervalMins = syncIntervalMins
			cfg.CaptureDir = tmpDir

			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)

			// Capture an image, then close.
			mockClock.Add(captureInterval)
			waitForCaptureFilesToExceedNFiles(tmpDir, 0)
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Get all captured data.
			_, capturedData, err := getCapturedData(tmpDir)
			test.That(t, err, test.ShouldBeNil)

			// Turn dmsvc back on with capture disabled.
			newDMSvc, r := newTestDataManager(t)
			defer newDMSvc.Close(context.Background())
			f := atomic.Bool{}
			f.Store(tc.serviceFail)
			mockClient := mockDataSyncServiceClient{
				streamingDCUploads: make(chan *mockStreamingDCClient, 10),
				fail:               &f,
			}
			// Set max unary file size to 1 byte, so it uses the streaming rpc.
			datasync.MaxUnaryFileSize = 1
			newDMSvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))
			cfg.CaptureDisabled = true
			cfg.ScheduledSyncDisabled = true
			resources = resourcesFromDeps(t, r, deps)
			err = newDMSvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)

			// Call sync.
			err = newDMSvc.Sync(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)

			// Wait for upload requests.
			var uploads []*mockStreamingDCClient
			var urs []*v1.StreamingDataCaptureUploadRequest
			// Get the successful requests
			wait := time.After(time.Second * 3)
			select {
			case <-wait:
				if !tc.serviceFail {
					t.Fatalf("timed out waiting for sync request")
				}
			case r := <-mockClient.streamingDCUploads:
				uploads = append(uploads, r)
				select {
				case <-wait:
					t.Fatalf("timed out waiting for sync request")
				case <-r.closed:
					urs = append(urs, r.reqs...)
				}
			}
			waitUntilNoFiles(tmpDir)

			// Validate error and URs.
			remainingFiles := getAllFilePaths(tmpDir)
			if tc.serviceFail {
				// Validate no files were successfully uploaded.
				test.That(t, len(uploads), test.ShouldEqual, 0)
				// Error case, file should not be deleted.
				test.That(t, len(remainingFiles), test.ShouldEqual, 1)
			} else {
				// Validate first metadata message.
				test.That(t, len(uploads), test.ShouldEqual, 1)
				test.That(t, len(urs), test.ShouldBeGreaterThan, 0)
				actMD := urs[0].GetMetadata()
				test.That(t, actMD, test.ShouldNotBeNil)
				test.That(t, actMD.GetUploadMetadata(), test.ShouldNotBeNil)
				test.That(t, actMD.GetSensorMetadata(), test.ShouldNotBeNil)
				test.That(t, actMD.GetUploadMetadata().Type, test.ShouldEqual, v1.DataType_DATA_TYPE_BINARY_SENSOR)
				test.That(t, actMD.GetUploadMetadata().PartId, test.ShouldNotBeBlank)

				// Validate ensuing data messages.
				dataRequests := urs[1:]
				var actData []byte
				for _, d := range dataRequests {
					actData = append(actData, d.GetData()...)
				}
				test.That(t, actData, test.ShouldResemble, capturedData[0].GetBinary())

				// Validate file no longer exists.
				test.That(t, len(getAllFileInfos(tmpDir)), test.ShouldEqual, 0)
			}
			test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
		})
	}
}

func TestSyncConfigUpdateBehavior(t *testing.T) {
	newSyncIntervalMins := 0.009
	tests := []struct {
		name                 string
		initSyncDisabled     bool
		initSyncIntervalMins float64
		newSyncDisabled      bool
		newSyncIntervalMins  float64
	}{
		{
			name:                 "all sync config stays the same, syncer should not cancel, ticker stays the same",
			initSyncDisabled:     false,
			initSyncIntervalMins: syncIntervalMins,
			newSyncDisabled:      false,
			newSyncIntervalMins:  syncIntervalMins,
		},
		{
			name:                 "sync config changes, new ticker should be created for sync",
			initSyncDisabled:     false,
			initSyncIntervalMins: syncIntervalMins,
			newSyncDisabled:      false,
			newSyncIntervalMins:  newSyncIntervalMins,
		},
		{
			name:                 "sync gets disabled, syncer should be nil",
			initSyncDisabled:     false,
			initSyncIntervalMins: syncIntervalMins,
			newSyncDisabled:      true,
			newSyncIntervalMins:  syncIntervalMins,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			mockClock := clk.NewMock()
			// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
			clock = mockClock
			tmpDir := t.TempDir()

			// Set up data manager.
			dmsvc, r := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			mockClient := mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fail:                &atomic.Bool{},
			}
			dmsvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))
			cfg, deps := setupConfig(t, enabledBinaryCollectorConfigPath)

			// Set up service config.
			cfg.CaptureDisabled = false
			cfg.ScheduledSyncDisabled = tc.initSyncDisabled
			cfg.CaptureDir = tmpDir
			cfg.SyncIntervalMins = tc.initSyncIntervalMins

			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)

			builtInSvc := dmsvc.(*builtIn)
			initTicker := builtInSvc.syncTicker

			// Reconfigure the dmsvc with new sync configs
			cfg.ScheduledSyncDisabled = tc.newSyncDisabled
			cfg.SyncIntervalMins = tc.newSyncIntervalMins

			err = dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: cfg,
			})
			test.That(t, err, test.ShouldBeNil)

			newBuildInSvc := dmsvc.(*builtIn)
			newTicker := newBuildInSvc.syncTicker
			newSyncer := newBuildInSvc.syncer

			if tc.newSyncDisabled {
				test.That(t, newSyncer, test.ShouldBeNil)
			}

			if tc.initSyncDisabled != tc.newSyncDisabled ||
				tc.initSyncIntervalMins != tc.newSyncIntervalMins {
				test.That(t, initTicker, test.ShouldNotEqual, newTicker)
			}
		})
	}
}

func getAllFilePaths(dir string) []string {
	var filePaths []string

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// ignore errors/unreadable files and directories
			//nolint:nilerr
			return nil
		}
		filePaths = append(filePaths, path)
		return nil
	})
	return filePaths
}

func getCapturedData(dir string) (int, []*v1.SensorData, error) {
	var allFiles []*datacapture.File
	filePaths := getAllFilePaths(dir)
	var numFiles int

	for _, f := range filePaths {
		osFile, err := os.Open(f)
		if err != nil {
			return 0, nil, err
		}
		dcFile, err := datacapture.ReadFile(osFile)
		if err != nil {
			return 0, nil, err
		}
		allFiles = append(allFiles, dcFile)
	}

	var ret []*v1.SensorData
	for _, dcFile := range allFiles {
		containsData := false
		for {
			next, err := dcFile.ReadNext()
			if errors.Is(err, io.EOF) {
				break
			}
			containsData = true
			if err != nil {
				return 0, nil, err
			}
			ret = append(ret, next)
		}
		if containsData {
			numFiles++
		}
	}
	return numFiles, ret, nil
}

func getUploadedData(urs []*v1.DataCaptureUploadRequest) []*v1.SensorData {
	var syncedData []*v1.SensorData
	for _, ur := range urs {
		sd := ur.GetSensorContents()
		syncedData = append(syncedData, sd...)
	}
	return syncedData
}

func compareSensorData(t *testing.T, dataType v1.DataType, act, exp []*v1.SensorData) {
	if len(act) == 0 && len(exp) == 0 {
		return
	}

	// Sort both by time requested.
	sort.SliceStable(act, func(i, j int) bool {
		diffRequested := act[j].GetMetadata().GetTimeRequested().AsTime().Sub(act[i].GetMetadata().GetTimeRequested().AsTime())
		switch {
		case diffRequested > 0:
			return true
		case diffRequested == 0:
			return act[j].GetMetadata().GetTimeReceived().AsTime().Sub(act[i].GetMetadata().GetTimeReceived().AsTime()) > 0
		default:
			return false
		}
	})
	sort.SliceStable(exp, func(i, j int) bool {
		diffRequested := exp[j].GetMetadata().GetTimeRequested().AsTime().Sub(exp[i].GetMetadata().GetTimeRequested().AsTime())
		switch {
		case diffRequested > 0:
			return true
		case diffRequested == 0:
			return exp[j].GetMetadata().GetTimeReceived().AsTime().Sub(exp[i].GetMetadata().GetTimeReceived().AsTime()) > 0
		default:
			return false
		}
	})

	test.That(t, len(act), test.ShouldEqual, len(exp))

	for i := range act {
		test.That(t, act[i].GetMetadata(), test.ShouldResemble, exp[i].GetMetadata())
		if dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
			test.That(t, act[i].GetStruct(), test.ShouldResemble, exp[i].GetStruct())
		} else {
			test.That(t, act[i].GetBinary(), test.ShouldResemble, exp[i].GetBinary())
		}
	}
}

type mockDataSyncServiceClient struct {
	succesfulDCRequests chan *v1.DataCaptureUploadRequest
	failedDCRequests    chan *v1.DataCaptureUploadRequest
	fileUploads         chan *mockFileUploadClient
	streamingDCUploads  chan *mockStreamingDCClient
	fail                *atomic.Bool
}

func (c mockDataSyncServiceClient) DataCaptureUpload(
	ctx context.Context,
	ur *v1.DataCaptureUploadRequest,
	opts ...grpc.CallOption,
) (*v1.DataCaptureUploadResponse, error) {
	if c.fail.Load() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case c.failedDCRequests <- ur:
			return nil, errors.New("oh no error")
		}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case c.succesfulDCRequests <- ur:
		return &v1.DataCaptureUploadResponse{}, nil
	}
}

func (c mockDataSyncServiceClient) FileUpload(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error) {
	if c.fail.Load() {
		return nil, errors.New("oh no error")
	}
	ret := &mockFileUploadClient{closed: make(chan struct{})}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case c.fileUploads <- ret:
	}
	return ret, nil
}

func (c mockDataSyncServiceClient) StreamingDataCaptureUpload(ctx context.Context,
	opts ...grpc.CallOption,
) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
	if c.fail.Load() {
		return nil, errors.New("oh no error")
	}
	ret := &mockStreamingDCClient{closed: make(chan struct{})}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case c.streamingDCUploads <- ret:
	}
	return ret, nil
}

type mockFileUploadClient struct {
	urs    []*v1.FileUploadRequest
	closed chan struct{}
	grpc.ClientStream
}

func (m *mockFileUploadClient) Send(req *v1.FileUploadRequest) error {
	m.urs = append(m.urs, req)
	return nil
}

func (m *mockFileUploadClient) CloseAndRecv() (*v1.FileUploadResponse, error) {
	m.closed <- struct{}{}
	return &v1.FileUploadResponse{}, nil
}

func (m *mockFileUploadClient) CloseSend() error {
	m.closed <- struct{}{}
	return nil
}

type mockStreamingDCClient struct {
	reqs   []*v1.StreamingDataCaptureUploadRequest
	closed chan struct{}
	grpc.ClientStream
}

func (m *mockStreamingDCClient) Send(req *v1.StreamingDataCaptureUploadRequest) error {
	m.reqs = append(m.reqs, req)
	return nil
}

func (m *mockStreamingDCClient) CloseAndRecv() (*v1.StreamingDataCaptureUploadResponse, error) {
	m.closed <- struct{}{}
	return &v1.StreamingDataCaptureUploadResponse{}, nil
}

func (m *mockStreamingDCClient) CloseSend() error {
	m.closed <- struct{}{}
	return nil
}

func getTestSyncerConstructorMock(client mockDataSyncServiceClient) datasync.ManagerConstructor {
	return func(identity string, _ v1.DataSyncServiceClient, logger logging.Logger, viamCaptureDotDir string) (datasync.Manager, error) {
		return datasync.NewManager(identity, client, logger, viamCaptureDotDir)
	}
}

func waitUntilNoFiles(dir string) {
	totalWait := time.Second * 3
	waitPerCheck := time.Millisecond * 10
	iterations := int(totalWait / waitPerCheck)
	files := getAllFileInfos(dir)
	for i := 0; i < iterations; i++ {
		if len(files) == 0 {
			return
		}
		time.Sleep(waitPerCheck)
		files = getAllFileInfos(dir)
	}
}

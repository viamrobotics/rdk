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
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/test"
	"google.golang.org/grpc"
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
			// Set up server.
			mockClock := clk.NewMock()
			// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
			clock = mockClock
			tmpDir := t.TempDir()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			var mockClient = mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fail:                &atomic.Bool{},
			}
			dmsvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))
			cfg := setupConfig(t, enabledBinaryCollectorConfigPath)

			// Set up service config.
			originalSvcConfig, ok1, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			originalSvcConfig.CaptureDisabled = false
			originalSvcConfig.ScheduledSyncDisabled = tc.initialServiceDisableStatus
			originalSvcConfig.CaptureDir = tmpDir
			originalSvcConfig.SyncIntervalMins = syncIntervalMins

			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)
			mockClock.Add(captureInterval)
			waitForCaptureFiles(tmpDir)
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
			updatedSvcConfig, ok2, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok2, test.ShouldBeTrue)
			updatedSvcConfig.CaptureDisabled = false
			updatedSvcConfig.ScheduledSyncDisabled = tc.newServiceDisableStatus
			updatedSvcConfig.CaptureDir = tmpDir
			updatedSvcConfig.SyncIntervalMins = syncIntervalMins

			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// Drain any requests that were already sent before Update returned.
			for len(mockClient.succesfulDCRequests) > 0 {
				<-mockClient.succesfulDCRequests
			}
			var sentReqAfterUpdate bool
			mockClock.Add(captureInterval)
			waitForCaptureFiles(tmpDir)
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
			name:     "Previously captured tabular data should be synced at start up.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		},
		{
			name:     "Previously captured binary data should be synced at start up.",
			dataType: v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name:                  "Manual sync should successfully sync captured tabular data.",
			dataType:              v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync:            true,
			scheduledSyncDisabled: true,
		},
		{
			name:                  "Manual sync should successfully sync captured binary data.",
			dataType:              v1.DataType_DATA_TYPE_BINARY_SENSOR,
			manualSync:            true,
			scheduledSyncDisabled: true,
		},
		{
			name:       "Running manual and scheduled sync concurrently should not cause data races or duplicate uploads.",
			dataType:   v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync: true,
		},
		{
			name:            "If tabular uploads fail transiently, they should be retried until they succeed.",
			dataType:        v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			failTransiently: true,
		},
		{
			name:            "If binary uploads fail transiently, they should be retried until they succeed.",
			dataType:        v1.DataType_DATA_TYPE_BINARY_SENSOR,
			failTransiently: true,
		},
		{
			name:      "Files with no sensor data should not be synced.",
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
			dmsvc := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			var cfg *config.Config
			captureInterval := time.Millisecond * 10
			if tc.emptyFile {
				cfg = setupConfig(t, infrequentCaptureTabularCollectorConfigPath)
			} else {
				if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
					cfg = setupConfig(t, enabledTabularCollectorConfigPath)
				} else {
					cfg = setupConfig(t, enabledBinaryCollectorConfigPath)
				}
			}

			// Set up service config with only capture enabled.
			svcConfig, ok1, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			svcConfig.CaptureDisabled = false
			svcConfig.ScheduledSyncDisabled = true
			svcConfig.SyncIntervalMins = syncIntervalMins
			svcConfig.CaptureDir = tmpDir

			err = dmsvc.Update(context.Background(), cfg)
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
			newDMSvc := newTestDataManager(t)
			defer newDMSvc.Close(context.Background())
			var mockClient = mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fail:                &atomic.Bool{},
			}
			newDMSvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))
			svcConfig.CaptureDisabled = true
			svcConfig.ScheduledSyncDisabled = tc.scheduledSyncDisabled
			svcConfig.SyncIntervalMins = syncIntervalMins
			err = newDMSvc.Update(context.Background(), cfg)
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
			waitUntilNoCaptureFiles := func(captureDir string) {
				totalWait := time.Second * 3
				waitPerCheck := time.Millisecond * 10
				iterations := int(totalWait / waitPerCheck)
				files := getAllFileInfos(captureDir)
				for i := 0; i < iterations; i++ {
					if len(files) == 0 {
						return
					}
					time.Sleep(waitPerCheck)
					files = getAllFileInfos(captureDir)
				}
			}
			waitUntilNoCaptureFiles(tmpDir)
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

// TODO DATA-1268: Reimplement this test.
// Cases: manual sync, successful, unsuccessful
// To validate: uploaded all data once, deleted file if no error, builds additional sync paths if none exist
func TestArbitraryFileUpload(t *testing.T) {
	t.Skip()
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	datasync.RetryExponentialFactor.Store(int32(1))
	datasync.InitialWaitTimeMillis.Store(int32(20))
	syncTime := time.Millisecond * 100
	datasync.UploadChunkSize = 1024
	fileName := "some_file_name.txt"
	// fileExt := ".txt"

	tests := []struct {
		name                 string
		manualSync           bool
		scheduleSyncDisabled bool
		serviceFail          bool
	}{
		{
			name:                 "Scheduled sync of arbitrary files should work.",
			manualSync:           false,
			scheduleSyncDisabled: false,
			serviceFail:          false,
		},
		{
			name:                 "Manual sync of arbitrary files should work.",
			manualSync:           true,
			scheduleSyncDisabled: true,
			serviceFail:          false,
		},
		{
			name:                 "Running manual and scheduled sync concurrently should work and not lead to duplicate uploads.",
			manualSync:           true,
			scheduleSyncDisabled: false,
			serviceFail:          false,
		},
		{
			name:                 "If an error response is received from the backend, local files should not be deleted.",
			manualSync:           false,
			scheduleSyncDisabled: false,
			serviceFail:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			additionalPathsDir := t.TempDir()
			captureDir := t.TempDir()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			dmsvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockDataSyncServiceClient{}))
			cfg := setupConfig(t, enabledTabularCollectorConfigPath)

			// Set up service config.
			svcConfig, ok, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok, test.ShouldBeTrue)
			svcConfig.CaptureDisabled = true
			svcConfig.ScheduledSyncDisabled = tc.scheduleSyncDisabled
			svcConfig.SyncIntervalMins = syncIntervalMins
			svcConfig.AdditionalSyncPaths = []string{additionalPathsDir}
			svcConfig.CaptureDir = captureDir

			// Start dmsvc.
			dmsvc.SetWaitAfterLastModifiedMillis(10)
			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// Write some files to the path.
			fileContents := make([]byte, datasync.UploadChunkSize*4)
			fileContents = append(fileContents, []byte("happy cows come from california")...)
			tmpFile, err := os.Create(filepath.Join(additionalPathsDir, fileName))
			test.That(t, err, test.ShouldBeNil)
			_, err = tmpFile.Write(fileContents)
			test.That(t, err, test.ShouldBeNil)
			time.Sleep(time.Millisecond * 100)

			// Call manual sync if desired.
			if tc.manualSync {
				err = dmsvc.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
			}
			time.Sleep(syncTime)

			// Validate error and URs.
			remainingFiles := getAllFilePaths(additionalPathsDir)
			if tc.serviceFail {
				// Error case.
				test.That(t, len(remainingFiles), test.ShouldEqual, 1)
			} else {
				// Validate first metadata message.
				// test.That(t, len(mockService.getFileUploadRequests()), test.ShouldBeGreaterThan, 0)
				// actMD := mockService.getFileUploadRequests()[0].GetMetadata()
				// test.That(t, actMD, test.ShouldNotBeNil)
				// test.That(t, actMD.Type, test.ShouldEqual, v1.DataType_DATA_TYPE_FILE)
				// test.That(t, actMD.FileName, test.ShouldEqual, fileName)
				// test.That(t, actMD.FileExtension, test.ShouldEqual, fileExt)
				// test.That(t, actMD.GetPartId(), test.ShouldEqual, cfg.Cloud.ID)

				// Validate ensuing data messages.
				// actDataRequests := mockService.getFileUploadRequests()[1:]
				// var actData []byte
				// for _, d := range actDataRequests {
				// 	actData = append(actData, d.GetFileContents().GetData()...)
				// }
				// test.That(t, actData, test.ShouldResemble, fileContents)

				// Validate file no longer exists.
				test.That(t, len(remainingFiles), test.ShouldEqual, 0)
			}
			test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
		})
	}
}

func getAllFilePaths(dir string) []string {
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

func compareSensorData(t *testing.T, dataType v1.DataType, act []*v1.SensorData, exp []*v1.SensorData) {
	if len(act) == 0 && len(exp) == 0 {
		return
	}

	// Sort both by time requested.
	sort.SliceStable(act, func(i, j int) bool {
		diffRequested := act[j].GetMetadata().GetTimeRequested().AsTime().Sub(act[i].GetMetadata().GetTimeRequested().AsTime())
		if diffRequested > 0 {
			return true
		} else if diffRequested == 0 {
			return act[j].GetMetadata().GetTimeReceived().AsTime().Sub(act[i].GetMetadata().GetTimeReceived().AsTime()) > 0
		} else {
			return false
		}
	})
	sort.SliceStable(exp, func(i, j int) bool {
		diffRequested := exp[j].GetMetadata().GetTimeRequested().AsTime().Sub(exp[i].GetMetadata().GetTimeRequested().AsTime())
		if diffRequested > 0 {
			return true
		} else if diffRequested == 0 {
			return exp[j].GetMetadata().GetTimeReceived().AsTime().Sub(exp[i].GetMetadata().GetTimeReceived().AsTime()) > 0
		} else {
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
	fail                *atomic.Bool
}

func (c mockDataSyncServiceClient) DataCaptureUpload(ctx context.Context, ur *v1.DataCaptureUploadRequest, opts ...grpc.CallOption) (*v1.DataCaptureUploadResponse, error) {
	if c.fail.Load() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case c.failedDCRequests <- ur:
			return nil, errors.New("oh no error!!")
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
	return nil, errors.New("not implemented")
}

func getTestSyncerConstructorMock(client mockDataSyncServiceClient) datasync.ManagerConstructor {
	return func(logger golog.Logger, cfg *config.Config) (datasync.Manager, error) {
		return datasync.NewManager(logger, cfg.Cloud.ID, client, nil)
	}
}

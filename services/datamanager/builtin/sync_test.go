package builtin

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
)

const (
	fiftyMillis = 0.0008
	syncTime    = time.Millisecond * 150
)

var (
	testLastModifiedMillis = 10
)

// TODO DATA-849: Add a test that validates that sync interval is accurately respected.

func TestSyncEnabled(t *testing.T) {
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
			tmpDir := t.TempDir()
			rpcServer, mockService := buildAndStartLocalSyncServer(t, 0, 0)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			cfg := setupConfig(t, enabledBinaryCollectorConfigPath)

			// Set up service config.
			originalSvcConfig, ok1, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			originalSvcConfig.CaptureDisabled = false
			originalSvcConfig.ScheduledSyncDisabled = tc.initialServiceDisableStatus
			originalSvcConfig.CaptureDir = tmpDir
			originalSvcConfig.SyncIntervalMins = fiftyMillis

			err = dmsvc.Update(context.Background(), cfg)

			// Let run for a second, then change status.
			time.Sleep(syncTime)

			initialUploadCount := len(mockService.getSuccessfulDCUploadRequests())
			if !tc.initialServiceDisableStatus {
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
			updatedSvcConfig.SyncIntervalMins = fiftyMillis

			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// Let run for a second, then change status.
			time.Sleep(syncTime)
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			newUploadCount := len(mockService.getSuccessfulDCUploadRequests())
			if !tc.newServiceDisableStatus {
				test.That(t, newUploadCount, test.ShouldBeGreaterThan, initialUploadCount)
			} else {
				// +1 to give leeway if an additional upload call was made between measuring initialUploadCount
				// and calling Update
				test.That(t, newUploadCount, test.ShouldBeBetweenOrEqual, initialUploadCount, initialUploadCount+1)
			}
		})
	}
}

// TODO DATA-849: Test concurrent capture and sync more thoroughly.
func TestDataCaptureUpload(t *testing.T) {
	datacapture.MaxFileSize = 500
	// MaxFileSize of 500 => Should be 2 tabular readings per file/UR, because the SensorReadings are ~230 bytes each
	sensorDataPerUploadRequest := 2.0
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	datasync.RetryExponentialFactor.Store(int32(1))
	datasync.InitialWaitTimeMillis.Store(int32(20))
	captureTime := time.Millisecond * 300

	tests := []struct {
		name                  string
		dataType              v1.DataType
		manualSync            bool
		scheduledSyncDisabled bool
		serviceFailAt         int
		numFails              int
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
			name:          "If tabular sync fails part way through, it should be resumed without duplicate uploads",
			dataType:      v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			serviceFailAt: 2,
		},
		{
			name:          "If binary sync fails part way through, it should be resumed without duplicate uploads",
			dataType:      v1.DataType_DATA_TYPE_BINARY_SENSOR,
			serviceFailAt: 2,
		},
		{
			name:     "If tabular uploads fail transiently, they should be retried until they succeed.",
			dataType: v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			numFails: 2,
		},
		{
			name:     "If binary uploads fail transiently, they should be retried until they succeed.",
			dataType: v1.DataType_DATA_TYPE_BINARY_SENSOR,
			numFails: 2,
		},
		{
			name:      "Files with no sensor data should not be synced.",
			emptyFile: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			tmpDir := t.TempDir()
			rpcServer, mockService := buildAndStartLocalSyncServer(t, tc.serviceFailAt, tc.numFails)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			var cfg *config.Config
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
			svcConfig.SyncIntervalMins = fiftyMillis
			svcConfig.CaptureDir = tmpDir

			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// Let run for a bit, then close.
			time.Sleep(captureTime)
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Get all captured data.
			capturedData, err := getCapturedData(tmpDir)
			test.That(t, err, test.ShouldBeNil)
			if tc.emptyFile {
				test.That(t, len(capturedData), test.ShouldEqual, 0)
			} else {
				test.That(t, len(capturedData), test.ShouldBeGreaterThan, 0)
			}

			// Turn dmsvc back on with capture disabled.
			newDMSvc := newTestDataManager(t)
			defer newDMSvc.Close(context.Background())
			newDMSvc.SetWaitAfterLastModifiedMillis(testLastModifiedMillis)
			newDMSvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			svcConfig.CaptureDisabled = true
			svcConfig.ScheduledSyncDisabled = tc.scheduledSyncDisabled
			svcConfig.SyncIntervalMins = fiftyMillis
			time.Sleep(time.Duration(testLastModifiedMillis) * time.Millisecond)
			err = newDMSvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// If testing manual sync, call sync. Call it multiple times to ensure concurrent calls are safe.
			if tc.manualSync {
				err = newDMSvc.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
				err = newDMSvc.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
			}
			time.Sleep(syncTime)
			err = newDMSvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// If we set failAt, we want to restart the service to simulate when the backend fails.
			var newMockService *mockDataSyncServiceServer
			if tc.serviceFailAt > 0 {
				// Now turn back on with only sync enabled.
				test.That(t, rpcServer.Stop(), test.ShouldBeNil)
				rpcServer, newMockService = buildAndStartLocalSyncServer(t, 0, 0)
				defer func() {
					err := rpcServer.Stop()
					test.That(t, err, test.ShouldBeNil)
				}()

				newestDMSvc := newTestDataManager(t)
				defer newestDMSvc.Close(context.Background())
				newestDMSvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
				newestDMSvc.SetWaitAfterLastModifiedMillis(testLastModifiedMillis)
				err = newestDMSvc.Update(context.Background(), cfg)
				test.That(t, err, test.ShouldBeNil)
				time.Sleep(syncTime)
				err = newestDMSvc.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)
			}

			// Calculate combined successful/failed requests
			var successfulURs []*v1.DataCaptureUploadRequest
			if tc.serviceFailAt > 0 {
				successfulURs = append(mockService.getSuccessfulDCUploadRequests(), newMockService.getSuccessfulDCUploadRequests()...)
			} else {
				successfulURs = mockService.getSuccessfulDCUploadRequests()
			}
			failedURs := mockService.getFailedDCUploadRequests()

			// If the server was supposed to fail for some requests, verify that it did.
			if tc.numFails != 0 || tc.serviceFailAt != 0 {
				test.That(t, len(failedURs), test.ShouldBeGreaterThan, 0)
			}

			// Validate expected number of upload requests were sent based on the quantity of data and chunking params.
			if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
				test.That(t, len(successfulURs), test.ShouldEqual, math.Ceil(float64(len(capturedData))/sensorDataPerUploadRequest))
			} else {
				test.That(t, len(successfulURs), test.ShouldEqual, len(capturedData))
			}

			// Validate that all captured data was synced.
			syncedData := getUploadedData(successfulURs)
			compareSensorData(t, tc.dataType, syncedData, capturedData)

			// After all uploads succeed, all files should be deleted.
			test.That(t, len(getAllFiles(tmpDir)), test.ShouldEqual, 0)
		})
	}
}

// Cases: manual sync, successful, unsuccessful
// To validate: uploaded all data once, deleted file if no error, builds additional sync paths if none exist
func TestArbitraryFileUpload(t *testing.T) {
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	datasync.RetryExponentialFactor.Store(int32(1))
	datasync.InitialWaitTimeMillis.Store(int32(20))
	syncTime := time.Millisecond * 100
	datasync.UploadChunkSize = 1024
	fileName := "some_file_name.txt"
	fileExt := ".txt"

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
			tmpDir := t.TempDir()

			var failFor int
			if tc.serviceFail {
				failFor = 100
			}
			rpcServer, mockService := buildAndStartLocalSyncServer(t, 0, failFor)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			defer dmsvc.Close(context.Background())
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			cfg := setupConfig(t, enabledTabularCollectorConfigPath)

			// Set up service config.
			svcConfig, ok, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok, test.ShouldBeTrue)
			svcConfig.CaptureDisabled = true
			svcConfig.ScheduledSyncDisabled = tc.scheduleSyncDisabled
			svcConfig.SyncIntervalMins = fiftyMillis
			svcConfig.AdditionalSyncPaths = []string{tmpDir}

			// Start dmsvc.
			dmsvc.SetWaitAfterLastModifiedMillis(testLastModifiedMillis)
			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// Write some files to the path.
			fileContents := make([]byte, datasync.UploadChunkSize*4)
			fileContents = append(fileContents, []byte("happy cows come from california")...)
			tmpFile, err := os.Create(filepath.Join(tmpDir, fileName))
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
			remainingFiles := getAllFilePaths(tmpDir)
			if tc.serviceFail {
				// Error case.
				test.That(t, len(remainingFiles), test.ShouldEqual, 1)
			} else {
				// Validate first metadata message.
				test.That(t, len(mockService.getFileUploadRequests()), test.ShouldBeGreaterThan, 0)
				actMD := mockService.getFileUploadRequests()[0].GetMetadata()
				test.That(t, actMD, test.ShouldNotBeNil)
				test.That(t, actMD.Type, test.ShouldEqual, v1.DataType_DATA_TYPE_FILE)
				test.That(t, actMD.FileName, test.ShouldEqual, fileName)
				test.That(t, actMD.FileExtension, test.ShouldEqual, fileExt)
				test.That(t, actMD.GetPartId(), test.ShouldEqual, cfg.Cloud.ID)

				// Validate ensuing data messages.
				actDataRequests := mockService.getFileUploadRequests()[1:]
				var actData []byte
				for _, d := range actDataRequests {
					actData = append(actData, d.GetFileContents().GetData()...)
				}
				test.That(t, actData, test.ShouldResemble, fileContents)

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

func getCapturedData(dir string) ([]*v1.SensorData, error) {
	var allFiles []*datacapture.File
	filePaths := getAllFilePaths(dir)

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
	if len(act) == 0 && len(exp) == 0 {
		return
	}

	// Sort both by time requested.
	sort.Slice(act, func(i, j int) bool {
		return act[i].GetMetadata().GetTimeRequested().Nanos < act[j].GetMetadata().GetTimeRequested().Nanos
	})
	sort.Slice(exp, func(i, j int) bool {
		return exp[i].GetMetadata().GetTimeRequested().Nanos < exp[j].GetMetadata().GetTimeRequested().Nanos
	})

	test.That(t, len(act), test.ShouldEqual, len(exp))
	if dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
		for i := range act {
			test.That(t, act[i].GetMetadata(), test.ShouldResemble, exp[i].GetMetadata())
			test.That(t, act[i].GetStruct(), test.ShouldResemble, exp[i].GetStruct())
		}
	} else {
		for i := range act {
			test.That(t, act[i].GetMetadata(), test.ShouldResemble, exp[i].GetMetadata())
			test.That(t, act[i].GetBinary(), test.ShouldResemble, exp[i].GetBinary())
		}
	}
}

func getTestSyncerConstructor(server rpc.Server) datasync.ManagerConstructor {
	return func(logger golog.Logger, cfg *config.Config, lastModMillis int) (datasync.Manager, error) {
		conn, err := getLocalServerConn(server, logger)
		if err != nil {
			return nil, err
		}
		client := datasync.NewClient(conn)
		return datasync.NewManager(logger, cfg.Cloud.ID, client, conn, lastModMillis)
	}
}

type mockDataSyncServiceServer struct {
	successfulDCUploadRequests *[]*v1.DataCaptureUploadRequest
	failedDCUploadRequests     *[]*v1.DataCaptureUploadRequest
	fileUploadRequests         *[]*v1.FileUploadRequest
	lock                       *sync.Mutex
	failAt                     int32
	failFor                    int32
	callCount                  *atomic.Int32
	v1.UnimplementedDataSyncServiceServer
}

func (m *mockDataSyncServiceServer) getSuccessfulDCUploadRequests() []*v1.DataCaptureUploadRequest {
	m.lock.Lock()
	defer m.lock.Unlock()
	return *m.successfulDCUploadRequests
}

func (m *mockDataSyncServiceServer) getFailedDCUploadRequests() []*v1.DataCaptureUploadRequest {
	m.lock.Lock()
	defer m.lock.Unlock()
	return *m.failedDCUploadRequests
}

func (m *mockDataSyncServiceServer) getFileUploadRequests() []*v1.FileUploadRequest {
	m.lock.Lock()
	defer m.lock.Unlock()
	return *m.fileUploadRequests
}

func (m *mockDataSyncServiceServer) setFailAt(v int32) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.failAt = v
}

func (m mockDataSyncServiceServer) DataCaptureUpload(ctx context.Context, ur *v1.DataCaptureUploadRequest) (*v1.DataCaptureUploadResponse, error) {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	defer m.callCount.Add(1)

	if m.failAt != 0 && m.callCount.Load() >= m.failAt {
		*m.failedDCUploadRequests = append(*m.failedDCUploadRequests, ur)
		return nil, errors.New("oh no error!!")
	}

	if m.failFor != 0 && m.callCount.Load() < m.failFor {
		*m.failedDCUploadRequests = append(*m.failedDCUploadRequests, ur)
		return nil, errors.New("oh no error!!")
	}
	*m.successfulDCUploadRequests = append(*m.successfulDCUploadRequests, ur)
	return &v1.DataCaptureUploadResponse{}, nil
}

func (m mockDataSyncServiceServer) FileUpload(stream v1.DataSyncService_FileUploadServer) error {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()

	if m.failFor > 0 {
		return errors.New("oh no, error")
	}

	for {
		ur, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			err := stream.SendAndClose(&v1.FileUploadResponse{})
			if err != nil {
				return err
			}
			break
		}
		if err != nil {
			return err
		}
		*m.fileUploadRequests = append(*m.fileUploadRequests, ur)
	}
	return nil
}

//nolint:thelper
func buildAndStartLocalSyncServer(t *testing.T, failAt int, failFor int) (rpc.Server, *mockDataSyncServiceServer) {
	logger := golog.NewTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	mockService := mockDataSyncServiceServer{
		successfulDCUploadRequests:         &[]*v1.DataCaptureUploadRequest{},
		failedDCUploadRequests:             &[]*v1.DataCaptureUploadRequest{},
		fileUploadRequests:                 &[]*v1.FileUploadRequest{},
		lock:                               &sync.Mutex{},
		UnimplementedDataSyncServiceServer: v1.UnimplementedDataSyncServiceServer{},
		failAt:                             int32(failAt),
		callCount:                          &atomic.Int32{},
		failFor:                            int32(failFor),
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

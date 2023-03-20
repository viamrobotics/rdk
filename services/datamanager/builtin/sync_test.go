package builtin

import (
	"context"
	"fmt"
	"io"
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
	syncTime    = time.Millisecond * 250
)

var (
	testLastModifiedMillis = 10
)

// TODO DATA-849: Add a test that validates that sync interval is accurately respected.
// TODO: what do we actually want to test? That the underlying sync.Manager.SyncDirectory() is called, and successfully
// finds + syncs all data in that dir
func TestSyncEnabled(t *testing.T) {
	waitForAndGetUploadRequests := func(srv *mockDataSyncServiceServer) []*v1.DataCaptureUploadRequest {
		var reqs []*v1.DataCaptureUploadRequest
		var added []*v1.DataCaptureUploadRequest
		for i := 0; i < 20; i++ {
			if len(reqs) > 0 && len(added) == 0 {
				srv.clear()
				return reqs
			}
			time.Sleep(time.Millisecond * 100)
			succ := srv.getSuccessfulDCUploadRequests()
			fmt.Println("succ", len(succ))
			added = []*v1.DataCaptureUploadRequest{}
			added = append(added, succ...)
			added = append(added, srv.getFailedDCUploadRequests()...)
			fmt.Println("added:", len(added))
			reqs = append(reqs, added...)
		}
		return reqs
	}

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
			// TODO: Instead of doing this, wait until both files have appeared and the sync.Manager has been called
			initialURs := waitForAndGetUploadRequests(mockService)

			if !tc.initialServiceDisableStatus {
				test.That(t, len(initialURs), test.ShouldBeGreaterThan, 0)
			} else {
				test.That(t, len(initialURs), test.ShouldEqual, 0)
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
			newUploads := waitForAndGetUploadRequests(mockService)
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			newUploadCount := len(newUploads)
			if !tc.newServiceDisableStatus {
				test.That(t, newUploadCount, test.ShouldBeGreaterThan, 0)
			} else {
				test.That(t, newUploadCount, test.ShouldEqual, 0)
			}
		})
	}
}

// TODO DATA-849: Test concurrent capture and sync more thoroughly.
func TestDataCaptureUpload(t *testing.T) {
	datacapture.MaxFileSize = 500
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

				newDMSvc.Close(context.Background())
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

			// Validate that all captured data was synced.
			syncedData := getUploadedData(successfulURs)
			compareSensorData(t, tc.dataType, syncedData, capturedData)

			// After all uploads succeed, all files should be deleted.
			test.That(t, len(getAllFileInfos(tmpDir)), test.ShouldEqual, 0)
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
			additionalPathsDir := t.TempDir()
			captureDir := t.TempDir()

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
			svcConfig.AdditionalSyncPaths = []string{additionalPathsDir}
			svcConfig.CaptureDir = captureDir

			// Start dmsvc.
			dmsvc.SetWaitAfterLastModifiedMillis(testLastModifiedMillis)
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

// TODO: Wrap this in some sort of signaling syncer
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
	fmt.Println("calling getSuccessfulDCUploadRequests", len(*m.successfulDCUploadRequests))
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

func (m *mockDataSyncServiceServer) clear() {
	m.lock.Lock()
	defer m.lock.Unlock()
	*m.successfulDCUploadRequests = nil
	*m.failedDCUploadRequests = nil
	*m.fileUploadRequests = nil
}

func (m mockDataSyncServiceServer) DataCaptureUpload(ctx context.Context, ur *v1.DataCaptureUploadRequest) (*v1.DataCaptureUploadResponse, error) {
	fmt.Println("Called DataCaptureUpload")

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
	fmt.Println("appending to successfulDCUploadRequests")
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

package builtin

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
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
	"go.viam.com/utils/rpc"
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
			rpcServer, mockService := buildAndStartLocalSyncServer(t)
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
			originalSvcConfig.SyncIntervalMins = syncIntervalMins

			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)
			mockClock.Add(captureInterval)
			waitForCaptureFiles(tmpDir)
			mockClock.Add(syncInterval)
			var receivedReq bool
			wait := time.After(time.Second)
			select {
			case <-wait:
			case <-mockService.succesfulDCRequests:
				receivedReq = true
			}

			if !tc.initialServiceDisableStatus {
				test.That(t, receivedReq, test.ShouldBeTrue)
			} else {
				test.That(t, receivedReq, test.ShouldBeFalse)
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

			// Drain any requests that were already sent.
			for len(mockService.succesfulDCRequests) > 0 {
				<-mockService.succesfulDCRequests
			}
			var receivedReqTwo bool
			mockClock.Add(captureInterval)
			waitForCaptureFiles(tmpDir)
			mockClock.Add(syncInterval)
			wait = time.After(time.Second)
			select {
			case <-wait:
			case <-mockService.succesfulDCRequests:
				receivedReqTwo = true
			}
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			if !tc.newServiceDisableStatus {
				test.That(t, receivedReqTwo, test.ShouldBeTrue)
			} else {
				test.That(t, receivedReqTwo, test.ShouldBeFalse)
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
			rpcServer, mockService := buildAndStartLocalSyncServer(t)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

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
			for i := 0; i < 5; i++ {
				mockClock.Add(captureInterval)
			}
			waitForCaptureFiles(tmpDir)
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
			newDMSvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			svcConfig.CaptureDisabled = true
			svcConfig.ScheduledSyncDisabled = tc.scheduledSyncDisabled
			svcConfig.SyncIntervalMins = syncIntervalMins
			err = newDMSvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			if tc.failTransiently {
				// Simulate the backend returning errors some number of times, and validate that the dmsvc is continuing
				// to retry.
				numFails := 3
				mockService.fail.Store(true)
				for i := 0; i < numFails; i++ {
					mockClock.Add(syncInterval)
					// Each time we sync, we should get a sync request for each file.
					for j := 0; j < numFiles; j++ {
						wait := time.After(time.Second * 5)
						select {
						case <-wait:
							t.Fatalf("timed out waiting for sync request")
						case <-mockService.failedDCRequests:
						}
					}
				}
			}

			mockService.fail.Store(false)
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
				case r := <-mockService.succesfulDCRequests:
					successfulReqs = append(successfulReqs, r)
				}
			}

			// Give it time to delete files after upload.
			waitUntilNoCaptureFiles := func(captureDir string) {
				files := getAllFileInfos(captureDir)
				for i := 0; i < 100; i++ {
					if len(files) == 0 {
						return
					}
					time.Sleep(time.Millisecond * 25)
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

			rpcServer, mockService := buildAndStartLocalSyncServer(t)
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
	return func(logger golog.Logger, cfg *config.Config) (datasync.Manager, error) {
		conn, err := getLocalServerConn(server, logger)
		if err != nil {
			return nil, err
		}
		client := datasync.NewClient(conn)
		return datasync.NewManager(logger, cfg.Cloud.ID, client, conn)
	}
}

type mockDataSyncServiceServer struct {
	succesfulDCRequests chan *v1.DataCaptureUploadRequest
	failedDCRequests    chan *v1.DataCaptureUploadRequest
	fileUploadRequests  *[]*v1.FileUploadRequest
	lock                *sync.Mutex
	fail                *atomic.Bool
	v1.UnimplementedDataSyncServiceServer
}

func (m *mockDataSyncServiceServer) getFileUploadRequests() []*v1.FileUploadRequest {
	m.lock.Lock()
	defer m.lock.Unlock()
	return *m.fileUploadRequests
}

func (m mockDataSyncServiceServer) DataCaptureUpload(ctx context.Context, ur *v1.DataCaptureUploadRequest) (*v1.DataCaptureUploadResponse, error) {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()

	if m.fail.Load() {
		m.failedDCRequests <- ur
		return nil, errors.New("oh no error!!")
	}
	m.succesfulDCRequests <- ur
	return &v1.DataCaptureUploadResponse{}, nil
}

func (m mockDataSyncServiceServer) FileUpload(stream v1.DataSyncService_FileUploadServer) error {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()

	if m.fail.Load() {
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
func buildAndStartLocalSyncServer(t *testing.T) (rpc.Server, *mockDataSyncServiceServer) {
	logger := golog.NewTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	mockService := mockDataSyncServiceServer{
		succesfulDCRequests:                make(chan *v1.DataCaptureUploadRequest, 100),
		failedDCRequests:                   make(chan *v1.DataCaptureUploadRequest, 100),
		fileUploadRequests:                 &[]*v1.FileUploadRequest{},
		lock:                               &sync.Mutex{},
		UnimplementedDataSyncServiceServer: v1.UnimplementedDataSyncServiceServer{},
		fail:                               &atomic.Bool{},
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

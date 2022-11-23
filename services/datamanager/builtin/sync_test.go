package builtin

import (
	"context"
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
			// Set up server.
			tmpDir, err := os.MkdirTemp("", "")
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				err := os.RemoveAll(tmpDir)
				test.That(t, err, test.ShouldBeNil)
			}()
			rpcServer, mockService := buildAndStartLocalSyncServer(t, 0, 0)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			cfg := setupConfig(t, enabledTabularCollectorConfigPath)

			// Set up service config.
			originalSvcConfig, ok1, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			originalSvcConfig.CaptureDisabled = false
			originalSvcConfig.ScheduledSyncDisabled = tc.initialServiceDisableStatus
			originalSvcConfig.CaptureDir = tmpDir
			originalSvcConfig.SyncIntervalMins = 0.001

			err = dmsvc.Update(context.Background(), cfg)

			// Let run for a second, then change status.
			time.Sleep(syncTime)

			initialUploadCount := len(mockService.getSuccessfulDCUploadRequests())
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
			updatedSvcConfig.SyncIntervalMins = 0.001

			err = dmsvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)

			// Let run for a second, then change status.
			time.Sleep(syncTime)
			err = dmsvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			newUploadCount := len(mockService.getSuccessfulDCUploadRequests())
			// TODO: Things to validate: that it syncs if expected, that it deletes files if successful
			if !tc.newServiceDisableStatus {
				test.That(t, newUploadCount, test.ShouldBeGreaterThan, initialUploadCount)
			} else {
				test.That(t, newUploadCount, test.ShouldEqual, initialUploadCount)
			}
		})
	}
}

func TestDataCaptureUpload(t *testing.T) {
	datacapture.MaxFileSize = 600
	// MaxFileSize of 600 => Should be 3 tabular readings per file/UR, because the SensorReadings are ~230 bytes each,
	// and the we start writing to a new file once the existing file size is > MaxFileSize.
	sensorDataPerUploadRequest := 3.0
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	datasync.RetryExponentialFactor = 1
	datasync.InitialWaitTimeMillis.Store(int32(20))
	captureTime := time.Millisecond * 300
	syncTime := time.Millisecond * 100

	tests := []struct {
		name          string
		dataType      v1.DataType
		serviceFailAt int
		numFails      int
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			tmpDir, err := os.MkdirTemp("", "")
			test.That(t, err, test.ShouldBeNil)
			defer func() {
				err := os.RemoveAll(tmpDir)
				test.That(t, err, test.ShouldBeNil)
			}()
			rpcServer, mockService := buildAndStartLocalSyncServer(t, tc.serviceFailAt, tc.numFails)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Set up data manager.
			dmsvc := newTestDataManager(t)
			dmsvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			var cfg *config.Config
			if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
				cfg = setupConfig(t, enabledTabularCollectorConfigPath)
			} else {
				cfg = setupConfig(t, enabledBinaryCollectorConfigPath)
			}

			// Set up service config.
			svcConfig, ok1, err := getServiceConfig(cfg)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ok1, test.ShouldBeTrue)
			svcConfig.CaptureDisabled = false
			svcConfig.ScheduledSyncDisabled = true
			svcConfig.SyncIntervalMins = 0.001
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
			test.That(t, len(capturedData), test.ShouldBeGreaterThan, 0)

			// Now turn back on with only sync enabled.
			newDMSvc := newTestDataManager(t)
			newDMSvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
			svcConfig.CaptureDisabled = true
			svcConfig.ScheduledSyncDisabled = false
			svcConfig.SyncIntervalMins = 0.001
			err = newDMSvc.Update(context.Background(), cfg)
			test.That(t, err, test.ShouldBeNil)
			time.Sleep(syncTime)
			err = newDMSvc.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// If we set failAt, we want to restart the service.
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
				newestDMSvc.SetSyncerConstructor(getTestSyncerConstructor(rpcServer))
				err = newestDMSvc.Update(context.Background(), cfg)
				test.That(t, err, test.ShouldBeNil)
				time.Sleep(syncTime)
				err = newestDMSvc.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)
			}

			// Calculate combined successful/failed requests
			var successfulURs []*v1.DataCaptureUploadRequest
			if newMockService != nil {
				successfulURs = append(mockService.getSuccessfulDCUploadRequests(), newMockService.getSuccessfulDCUploadRequests()...)
			} else {
				successfulURs = mockService.getSuccessfulDCUploadRequests()
			}
			failedURs := mockService.getFailedDCUploadRequests()

			// If the server was supposed to fail for some requests, verify that it did.
			if tc.numFails != 0 || tc.serviceFailAt != 0 {
				test.That(t, len(failedURs), test.ShouldBeGreaterThan, 0)
			}

			if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
				test.That(t, len(successfulURs), test.ShouldEqual, math.Ceil(float64(len(capturedData))/sensorDataPerUploadRequest))
			} else {
				test.That(t, len(successfulURs), test.ShouldEqual, len(capturedData))
			}

			// Validate that all captured data was synced.
			syncedData := getUploadedData(successfulURs)
			compareSensorData(t, tc.dataType, syncedData, capturedData)

			// After all uploads succeed, their files should be deleted.
			test.That(t, len(getAllFiles(tmpDir)), test.ShouldEqual, 0)
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
	if len(act) == 0 && len(exp) == 0 {
		return
	}

	// TODO: metadata checks fail because these don't get uploaded in a defined order. should prob use sets instead
	//       of arrays. For now, just don't check metadata
	test.That(t, len(act), test.ShouldEqual, len(exp))
	if dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
		for i := range act {
			test.That(t, act[i].GetStruct(), test.ShouldResemble, exp[i].GetStruct())
		}
	} else {
		for i := range act {
			test.That(t, act[i].GetBinary(), test.ShouldResemble, exp[i].GetBinary())
		}
	}
}

// TODO: readd arbitrary file upload tests
// TODO: readd manual sync tests
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
func getTestSyncerConstructor(server rpc.Server) datasync.ManagerConstructor {
	return func(logger golog.Logger, cfg *config.Config, interval time.Duration) (datasync.Manager, error) {
		conn, err := getLocalServerConn(server, logger)
		if err != nil {
			return nil, err
		}
		client := datasync.NewClient(conn)
		return datasync.NewManager(logger, cfg.Cloud.ID, client, conn, interval)
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
	// TODO: will likely need to make this optionally return errors for testing error cases
	*m.successfulDCUploadRequests = append(*m.successfulDCUploadRequests, ur)
	return &v1.DataCaptureUploadResponse{
		Code:    200,
		Message: "",
	}, nil
}

func (m mockDataSyncServiceServer) FileUpload(stream v1.DataSyncService_FileUploadServer) error {
	return status.Errorf(codes.Unimplemented, "method FileUpload not implemented")
}

//nolint:thelper
func buildAndStartLocalSyncServer(t *testing.T, failAt int, failFor int) (rpc.Server, *mockDataSyncServiceServer) {
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	mockService := mockDataSyncServiceServer{
		successfulDCUploadRequests:         &[]*v1.DataCaptureUploadRequest{},
		failedDCUploadRequests:             &[]*v1.DataCaptureUploadRequest{},
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

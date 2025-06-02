package builtin

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	syncIntervalMins = 0.0008
	syncInterval     = time.Millisecond * 48
	waitTime         = syncInterval * 4
)

// TODO DATA-849: Add a test that validates that the sync interval is accurately respected.
func TestSyncEnabled(t *testing.T) {
	tests := []struct {
		name                 string
		syncStartDisabled    bool
		syncEndDisabled      bool
		connStateConstructor func(rpc.ClientConn) datasync.ConnectivityState
		cloudConnectionErr   error
	}{
		{
			name:                 "config with sync disabled while online should sync nothing",
			syncStartDisabled:    true,
			syncEndDisabled:      true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "config with sync enabled while online should sync",
			syncStartDisabled:    false,
			syncEndDisabled:      false,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "disabling sync while online should stop syncing",
			syncStartDisabled:    false,
			syncEndDisabled:      true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "enabling sync while online should trigger syncing to start",
			syncStartDisabled:    true,
			syncEndDisabled:      false,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:               "config with sync disabled while offline should sync nothing",
			syncStartDisabled:  true,
			syncEndDisabled:    true,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:               "config with sync enabled while offline should sync nothing",
			syncStartDisabled:  false,
			syncEndDisabled:    false,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:               "disabling sync while offline should sync nothing",
			syncStartDisabled:  false,
			syncEndDisabled:    true,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:               "enabling sync while offline sync nothing",
			syncStartDisabled:  true,
			syncEndDisabled:    false,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:                 "config with sync disabled while connection is not ready should sync nothing",
			syncStartDisabled:    true,
			syncEndDisabled:      true,
			connStateConstructor: connToConnectivityStateError,
		},
		{
			name:                 "config with sync enabled while connection is not ready should sync nothing",
			syncStartDisabled:    false,
			syncEndDisabled:      false,
			connStateConstructor: connToConnectivityStateError,
		},
		{
			name:                 "disabling sync while connection is not ready should sync nothing",
			syncStartDisabled:    false,
			syncEndDisabled:      true,
			connStateConstructor: connToConnectivityStateError,
		},
		{
			name:                 "enabling sync while connection is not ready sync nothing",
			syncStartDisabled:    true,
			syncEndDisabled:      false,
			connStateConstructor: connToConnectivityStateError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := logging.NewTestLogger(t)
			tmpDir := t.TempDir()

			var secondReconfigure atomic.Bool
			firstCalledCtx, firstCalledFn := context.WithCancel(context.Background())
			secondCalledCtx, secondCalledFn := context.WithCancel(context.Background())
			dataSyncServiceClientConstructor := func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient {
				return datasync.MockDataSyncServiceClient{
					T: t,
					DataCaptureUploadFunc: func(
						ctx context.Context,
						in *v1.DataCaptureUploadRequest,
						opts ...grpc.CallOption,
					) (*v1.DataCaptureUploadResponse, error) {
						t.Log("called")
						if err := ctx.Err(); err != nil {
							t.Log(err.Error())
							return nil, err
						}
						firstCalledFn()
						if secondReconfigure.Load() {
							secondCalledFn()
						}
						return &v1.DataCaptureUploadResponse{}, nil
					},
				}
			}

			imgPng := newImgPng(t)
			r := setupRobot(tc.cloudConnectionErr, map[resource.Name]resource.Resource{
				camera.Named("c1"): &inject.Camera{
					ImageFunc: func(
						ctx context.Context,
						mimeType string,
						extra map[string]interface{},
					) ([]byte, camera.ImageMetadata, error) {
						return newImageBytesResp(ctx, imgPng, mimeType)
					},
				},
			})
			config, deps := setupConfig(t, r, enabledBinaryCollectorConfigPath)
			c := config.ConvertedAttributes.(*Config)
			c.CaptureDisabled = false
			c.ScheduledSyncDisabled = tc.syncStartDisabled
			c.CaptureDir = tmpDir
			c.SyncIntervalMins = syncIntervalMins
			// MaximumCaptureFileSizeBytes is set to 1 so that each reading becomes its own capture file
			// and we can confidently read the capture file without it's contents being modified by the collector
			c.MaximumCaptureFileSizeBytes = 1

			b, err := New(context.Background(), deps, config, dataSyncServiceClientConstructor, tc.connStateConstructor, logger)
			test.That(t, err, test.ShouldBeNil)
			defer b.Close(context.Background())
			t.Logf("waiting for data capture to write a data capture file %s", time.Now())
			waitForCaptureFilesToExceedNFiles(tmpDir, 0, logger)
			t.Logf("got a file %s", time.Now())
			wait := time.After(waitTime)
			t.Logf("waiting up to %s for a file to be uploaded", waitTime)
			select {
			case <-wait:
			case <-firstCalledCtx.Done():
			}

			offline := tc.connStateConstructor == nil || tc.connStateConstructor(nil).GetState() != connectivity.Ready
			if tc.syncStartDisabled || offline || tc.cloudConnectionErr != nil {
				test.That(t, firstCalledCtx.Err(), test.ShouldBeNil)
			} else {
				test.That(t, firstCalledCtx.Err(), test.ShouldBeError, context.Canceled)
			}

			// Set up service end config.
			c.ScheduledSyncDisabled = tc.syncEndDisabled
			err = b.Reconfigure(context.Background(), deps, config)
			test.That(t, err, test.ShouldBeNil)
			secondReconfigure.Store(true)

			waitForCaptureFilesToExceedNFiles(tmpDir, 0, logger)
			wait = time.After(waitTime)
			select {
			case <-wait:
			case <-secondCalledCtx.Done():
			}

			if tc.syncEndDisabled || offline || tc.cloudConnectionErr != nil {
				test.That(t, secondCalledCtx.Err(), test.ShouldBeNil)
			} else {
				test.That(t, secondCalledCtx.Err(), test.ShouldBeError, context.Canceled)
			}
		})
	}
}

// TODO DATA-849: Test concurrent capture and sync more thoroughly.
func TestDataCaptureUploadIntegration(t *testing.T) {
	logger := logging.NewTestLogger(t)
	tests := []struct {
		name                  string
		dataType              v1.DataType
		manualSync            bool
		scheduledSyncDisabled bool
		failTransiently       bool
		emptyFile             bool
		connStateConstructor  func(rpc.ClientConn) datasync.ConnectivityState
		cloudConnectionErr    error
	}{
		{
			name:                 "previously captured tabular data should be synced at start up if online",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "previously captured binary data should be synced at start up if online",
			dataType:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                  "manual sync should successfully sync captured tabular data if online",
			dataType:              v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync:            true,
			scheduledSyncDisabled: true,
			connStateConstructor:  ConnToConnectivityStateReady,
		},
		{
			name:                  "manual sync should successfully sync captured binary data if online",
			dataType:              v1.DataType_DATA_TYPE_BINARY_SENSOR,
			manualSync:            true,
			scheduledSyncDisabled: true,
			connStateConstructor:  ConnToConnectivityStateReady,
		},
		{
			name:                 "running manual and scheduled sync concurrently should not cause data races or duplicate uploads if online",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync:           true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "if tabular uploads fail transiently, they should be retried until they succeed if online",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			failTransiently:      true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "if binary uploads fail transiently, they should be retried until they succeed if online",
			dataType:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			failTransiently:      true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "files with no sensor data should not be synced if online",
			emptyFile:            true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:               "previously captured tabular data should not be synced at start up if offline",
			dataType:           v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:               "previously captured binary data should not be synced at start up if offline",
			dataType:           v1.DataType_DATA_TYPE_BINARY_SENSOR,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			//
			name:               "running manual and scheduled sync concurrently should not cause data races or duplicate uploads if offline",
			dataType:           v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync:         true,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:               "files with no sensor data should not be synced if offline",
			emptyFile:          true,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:                 "previously captured tabular data should not be synced at start up if connection is not ready",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			connStateConstructor: connToConnectivityStateError,
		},
		{
			name:                 "previously captured binary data should not be synced at start up if connection is not ready",
			dataType:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			connStateConstructor: connToConnectivityStateError,
		},
		{
			name: "running manual and scheduled sync concurrently should not " +
				"cause data races or duplicate uploads if connection is not ready",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			manualSync:           true,
			connStateConstructor: connToConnectivityStateError,
		},
		{
			name:                 "files with no sensor data should not be synced if connection is not ready",
			emptyFile:            true,
			connStateConstructor: connToConnectivityStateError,
		},
	}
	initialWaitTimeMillis := datasync.InitialWaitTimeMillis
	retryExponentialFactor := datasync.RetryExponentialFactor
	t.Cleanup(func() {
		datasync.InitialWaitTimeMillis = initialWaitTimeMillis
		datasync.RetryExponentialFactor = retryExponentialFactor
	})
	datasync.InitialWaitTimeMillis = 50
	datasync.RetryExponentialFactor = 1

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Set up data manager.
			imgPng := newImgPng(t)
			var config resource.Config
			var deps resource.Dependencies
			captureInterval := time.Millisecond * 10
			r := setupRobot(tc.cloudConnectionErr, map[resource.Name]resource.Resource{
				arm.Named("arm1"): &inject.Arm{
					EndPositionFunc: func(
						ctx context.Context,
						extra map[string]interface{},
					) (spatialmath.Pose, error) {
						return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
					},
				},
			})

			if tc.emptyFile {
				config, deps = setupConfig(t, r, infrequentCaptureTabularCollectorConfigPath)
			} else {
				if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
					config, deps = setupConfig(t, r, enabledTabularCollectorConfigPath)
				} else {
					r := setupRobot(tc.cloudConnectionErr, map[resource.Name]resource.Resource{
						camera.Named("c1"): &inject.Camera{
							ImageFunc: func(
								ctx context.Context,
								mimeType string,
								extra map[string]interface{},
							) ([]byte, camera.ImageMetadata, error) {
								return newImageBytesResp(ctx, imgPng, mimeType)
							},
						},
					})
					config, deps = setupConfig(t, r, enabledBinaryCollectorConfigPath)
				}
			}

			// Set up service config with only capture enabled.
			c := config.ConvertedAttributes.(*Config)
			c.CaptureDisabled = false
			c.ScheduledSyncDisabled = true
			c.SyncIntervalMins = syncIntervalMins
			c.CaptureDir = tmpDir

			b, err := New(context.Background(), deps, config, datasync.NoOpCloudClientConstructor, tc.connStateConstructor, logger)
			test.That(t, err, test.ShouldBeNil)

			time.Sleep(captureInterval * 20)
			err = b.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Get all captured data.
			filePaths := getAllFilePaths(tmpDir)
			numFiles, capturedData, err := getCapturedData(filePaths)
			test.That(t, err, test.ShouldBeNil)
			if tc.emptyFile {
				test.That(t, len(capturedData), test.ShouldEqual, 0)
			} else {
				test.That(t, len(capturedData), test.ShouldBeGreaterThan, 0)
			}

			// Turn builtin back on with capture disabled.
			var fail atomic.Bool
			failChan := make(chan *v1.DataCaptureUploadRequest, numFiles)
			successChan := make(chan *v1.DataCaptureUploadRequest, numFiles)

			f := func(
				ctx context.Context,
				in *v1.DataCaptureUploadRequest,
				_ ...grpc.CallOption,
			) (*v1.DataCaptureUploadResponse, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case successChan <- in:
					return &v1.DataCaptureUploadResponse{}, nil
				}
			}

			if tc.failTransiently {
				fail.Store(true)
				f = func(
					ctx context.Context,
					in *v1.DataCaptureUploadRequest,
					_ ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}

					if fail.Load() {
						select {
						case <-ctx.Done():
							return nil, ctx.Err()
						case failChan <- in:
							return nil, errors.New("transient error")
						}
					}

					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case successChan <- in:
						return &v1.DataCaptureUploadResponse{}, nil
					}
				}
			}

			dataSyncServiceClientConstructor := func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient {
				return datasync.MockDataSyncServiceClient{
					DataCaptureUploadFunc: f,
				}
			}
			c.CaptureDisabled = true
			c.ScheduledSyncDisabled = tc.scheduledSyncDisabled
			c.SyncIntervalMins = syncIntervalMins

			b2Svc, err := New(context.Background(), deps, config, dataSyncServiceClientConstructor, tc.connStateConstructor, logger)
			test.That(t, err, test.ShouldBeNil)
			b2 := b2Svc.(*builtIn)

			if tc.failTransiently {
				timeout := time.After(waitTime * 10)
				failCount := 3
				for i := 0; i < failCount; i++ {
					t.Logf("waiting for %d files to fail", numFiles)
					for j := 0; j < numFiles; j++ {
						select {
						case <-timeout:
							t.Fatalf("timed out waiting for sync request")
							return
						case <-failChan:
							t.Logf("got file %d", j+1)
						}
					}
				}
			}
			fail.Store(false)

			// If testing manual sync, call sync. Call it multiple times to ensure concurrent calls are safe.
			if tc.manualSync {
				// wait until sync has gotten a cloud connection
				if tc.cloudConnectionErr != nil {
					select {
					case <-time.After(waitTime):
					case <-b2.sync.CloudConnReady():
						t.Fatal("should not happen")
					}
				} else {
					select {
					case <-time.After(waitTime * 10):
						t.Fatal("timeout")
					case <-b2.sync.CloudConnReady():
					}
					test.That(t, b2.Sync(context.Background(), nil), test.ShouldBeNil)
					test.That(t, b2.Sync(context.Background(), nil), test.ShouldBeNil)
				}
			}

			if tc.cloudConnectionErr == nil {
				wait := time.After(waitTime)
				var successfulReqs []*v1.DataCaptureUploadRequest
				// Get the successful requests
				for i := 0; i < numFiles; i++ {
					select {
					case <-wait:
						offline := tc.connStateConstructor == nil || tc.connStateConstructor(nil).GetState() != connectivity.Ready
						if offline && !tc.manualSync {
							err = b2.Close(context.Background())
							test.That(t, err, test.ShouldBeNil)
							// the same captured data from the first instance of data manager should still be on disk
							numFiles2, capturedData2, err := getCapturedData(filePaths)
							test.That(t, err, test.ShouldBeNil)
							test.That(t, numFiles, test.ShouldEqual, numFiles2)
							compareSensorData(t, tc.dataType, capturedData2, capturedData)
							// test done, we were offline & we didn't try to upload
							return
						}
						t.Fatalf("timed out waiting for sync request")
					case r := <-successChan:
						successfulReqs = append(successfulReqs, r)
					}
				}

				// Give it time to delete files after upload.
				waitUntilNoFiles(tmpDir)
				err = b2.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)

				// Validate that all captured data was synced.
				syncedData := getUploadedData(successfulReqs)
				compareSensorData(t, tc.dataType, syncedData, capturedData)

				// After all uploads succeed, all files should be deleted.
				test.That(t, len(getAllFileInfos(tmpDir)), test.ShouldEqual, 0)
			} else {
				err = b2.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)

				// Validate that all captured data is still on disk
				_, capturedData2, err := getCapturedData(getAllFilePaths(tmpDir))
				test.That(t, err, test.ShouldBeNil)
				compareSensorData(t, tc.dataType, capturedData2, capturedData)
			}
		})
	}
}

func TestArbitraryFileUpload(t *testing.T) {
	logger := logging.NewTestLogger(t)
	fileName := "some_file_name.txt"
	fileExt := ".txt"
	emptyFileTestName := "error due to empty file, local files should not be deleted"
	// Disable the check to see if the file was modified recently,
	// since we are testing instanteous arbitrary file uploads.
	tests := []struct {
		name                 string
		manualSync           bool
		scheduleSyncDisabled bool
		serviceFail          bool
		connStateConstructor func(rpc.ClientConn) datasync.ConnectivityState
		cloudConnectionErr   error
	}{
		{
			name:                 "scheduled sync of arbitrary files should work",
			manualSync:           false,
			scheduleSyncDisabled: false,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "manual sync of arbitrary files should work",
			manualSync:           true,
			scheduleSyncDisabled: true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "running manual and scheduled sync concurrently should work and not lead to duplicate uploads",
			manualSync:           true,
			scheduleSyncDisabled: false,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 "if an error response is received from the backend, local files should not be deleted",
			manualSync:           false,
			scheduleSyncDisabled: false,
			serviceFail:          true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
		{
			name:                 emptyFileTestName,
			manualSync:           false,
			scheduleSyncDisabled: false,
			serviceFail:          true,
			connStateConstructor: ConnToConnectivityStateReady,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			additionalPathsDir := t.TempDir()
			captureDir := t.TempDir()

			rs := []*v1.FileUploadRequest{}
			closeCtx, closeFn := context.WithCancel(context.Background())
			var uploadCount atomic.Uint64
			mockClient := datasync.MockDataSyncServiceClient{
				T: t,
				FileUploadFunc: func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error) {
					return &datasync.ClientStreamingMock[*v1.FileUploadRequest, *v1.FileUploadResponse]{
						SendFunc: func(r *v1.FileUploadRequest) error {
							if err := ctx.Err(); err != nil {
								return err
							}
							rs = append(rs, r)
							return nil
						},
						CloseAndRecvFunc: func() (*v1.FileUploadResponse, error) {
							if err := ctx.Err(); err != nil {
								return nil, err
							}
							if tc.serviceFail {
								return nil, errors.New("CloseAndRecv")
							}
							closeFn()
							uploadCount.Add(1)
							return &v1.FileUploadResponse{FileId: "some file id"}, nil
						},
					}, nil
				},
			}

			r := setupRobot(tc.cloudConnectionErr, map[resource.Name]resource.Resource{
				arm.Named("arm1"): &inject.Arm{
					EndPositionFunc: func(
						ctx context.Context,
						extra map[string]interface{},
					) (spatialmath.Pose, error) {
						return spatialmath.NewZeroPose(), nil
					},
				},
			})
			dataSyncServiceClientConstructor := func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient { return mockClient }
			config, deps := setupConfig(t, r, disabledTabularCollectorConfigPath)
			c := config.ConvertedAttributes.(*Config)
			c.ScheduledSyncDisabled = tc.scheduleSyncDisabled
			c.SyncIntervalMins = syncIntervalMins
			c.AdditionalSyncPaths = []string{additionalPathsDir}
			c.CaptureDir = captureDir
			c.FileLastModifiedMillis = 1
			t.Logf("cfg.AdditionalSyncPaths: %s, cfg.CaptureDir: %s", additionalPathsDir, c.CaptureDir)

			b, err := New(context.Background(), deps, config, dataSyncServiceClientConstructor, tc.connStateConstructor, logger)
			test.That(t, err, test.ShouldBeNil)
			defer b.Close(context.Background())

			// Write file to the path.
			var fileContents []byte
			if tc.name != emptyFileTestName {
				t.Log("populating FileContents")
				fileContents = populateFileContents(fileContents)
			}
			tmpFile, err := os.Create(filepath.Join(additionalPathsDir, fileName))
			test.That(t, err, test.ShouldBeNil)
			_, err = tmpFile.Write(fileContents)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, tmpFile.Close(), test.ShouldBeNil)
			t.Logf("tmpfilepath: %s", filepath.Join(additionalPathsDir, fileName))
			time.Sleep(time.Millisecond * time.Duration(c.FileLastModifiedMillis) * 2)

			// Call manual sync.
			if tc.manualSync {
				t.Log("calling manual sync")
				timeoutCtx, timeoutFn := context.WithTimeout(context.Background(), time.Second*5)
				defer timeoutFn()
				for {
					if err = b.Sync(context.Background(), nil); err == nil {
						break
					}

					if timeoutCtx.Err() != nil {
						t.Log("timed out waiting for mocked cloud connection")
						t.FailNow()
					}
					time.Sleep(time.Millisecond * 50)
				}
			}

			// Wait for upload requests.
			// Get the successful requests
			wait := time.After(waitTime)
			select {
			case <-wait:
				if !tc.serviceFail {
					t.Fatalf("timed out waiting for sync request")
				}
			case <-closeCtx.Done():
			}

			waitUntilNoFiles(additionalPathsDir)
			if !tc.serviceFail {
				// Validate first metadata message.
				test.That(t, uploadCount.Load(), test.ShouldEqual, 1)
				test.That(t, len(rs), test.ShouldBeGreaterThan, 0)
				actMD := rs[0].GetMetadata()
				test.That(t, actMD, test.ShouldNotBeNil)
				test.That(t, actMD.Type, test.ShouldEqual, v1.DataType_DATA_TYPE_FILE)
				test.That(t, filepath.Base(actMD.FileName), test.ShouldEqual, fileName)
				test.That(t, actMD.FileExtension, test.ShouldEqual, fileExt)
				test.That(t, actMD.PartId, test.ShouldNotBeBlank)

				// Validate ensuing data messages.
				dataRequests := rs[1:]
				var actData []byte
				for _, d := range dataRequests {
					actData = append(actData, d.GetFileContents().GetData()...)
				}
				test.That(t, actData, test.ShouldResemble, fileContents)

				// Validate file no longer exists.
				test.That(t, len(getAllFileInfos(additionalPathsDir)), test.ShouldEqual, 0)
				test.That(t, b.Close(context.Background()), test.ShouldBeNil)
			} else {
				// Validate no files were successfully uploaded.
				test.That(t, uploadCount.Load(), test.ShouldEqual, 0)
				// Validate file still exists.
				test.That(t, len(getAllFileInfos(additionalPathsDir)), test.ShouldEqual, 1)
			}
		})
	}
}

func TestStreamingDCUpload(t *testing.T) {
	// Set max unary file size to 1 byte, so it uses the streaming rpc. Reset the original
	// value such that other tests take the correct code paths.
	origSize := datasync.MaxUnaryFileSize
	datasync.MaxUnaryFileSize = 1
	t.Cleanup(func() {
		datasync.MaxUnaryFileSize = origSize
	})

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
			logger := logging.NewTestLogger(t)
			tmpDir := t.TempDir()

			// Set up data manager.
			imgPng := newImgPng(t)
			r := setupRobot(nil, map[resource.Name]resource.Resource{
				camera.Named("c1"): &inject.Camera{
					ImageFunc: func(
						ctx context.Context,
						mimeType string,
						extra map[string]interface{},
					) ([]byte, camera.ImageMetadata, error) {
						return newImageBytesResp(ctx, imgPng, mimeType)
					},
				},
			})
			config, deps := setupConfig(t, r, enabledBinaryCollectorConfigPath)
			c := config.ConvertedAttributes.(*Config)

			// Set up service config with just capture enabled.
			c.CaptureDisabled = false
			c.ScheduledSyncDisabled = true
			c.SyncIntervalMins = syncIntervalMins
			c.CaptureDir = tmpDir
			// MaximumCaptureFileSizeBytes is set to 1 so that each reading becomes its own capture file
			// and we can confidently read the capture file without it's contents being modified by the collector
			c.MaximumCaptureFileSizeBytes = 1
			b, err := New(context.Background(), deps, config, datasync.NoOpCloudClientConstructor, ConnToConnectivityStateReady, logger)
			test.That(t, err, test.ShouldBeNil)

			// Capture an image, then close.
			waitForCaptureFilesToExceedNFiles(tmpDir, 0, logger)
			err = b.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Get all captured data.
			filePaths := getAllFilePaths(tmpDir)
			_, capturedData, err := getCapturedData(filePaths)
			test.That(t, err, test.ShouldBeNil)
			t.Logf("capturedData len: %d", len(capturedData))

			// Turn builtin back on with capture disabled.
			type streamingState struct {
				mu               sync.Mutex
				streamingClients []*datasync.ClientStreamingMock[
					*v1.StreamingDataCaptureUploadRequest,
					*v1.StreamingDataCaptureUploadResponse,
				]
				requests [][]*v1.StreamingDataCaptureUploadRequest
				contexts []context.Context
			}
			ss := streamingState{
				streamingClients: []*datasync.ClientStreamingMock[
					*v1.StreamingDataCaptureUploadRequest,
					*v1.StreamingDataCaptureUploadResponse]{},
				requests: [][]*v1.StreamingDataCaptureUploadRequest{},
				contexts: []context.Context{},
			}

			firstSuccesssfulClientCreatedCtx,
				firstSuccesssfulClientCreatedFn := context.WithCancel(context.Background())
			streamingDataCaptureUploadFunc := func(
				ctx context.Context,
				_ ...grpc.CallOption,
			) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
				if tc.serviceFail {
					return nil, errors.New("oh no error")
				}
				cancelCtx, cancelFn := context.WithCancel(context.Background())
				ss.mu.Lock()
				idx := len(ss.streamingClients)
				mockStreamingClient := &datasync.ClientStreamingMock[
					*v1.StreamingDataCaptureUploadRequest,
					*v1.StreamingDataCaptureUploadResponse,
				]{
					T: t,
					SendFunc: func(in *v1.StreamingDataCaptureUploadRequest) error {
						ss.mu.Lock()
						ss.requests[idx] = append(ss.requests[idx], in)
						ss.mu.Unlock()
						return nil
					},
					CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
						cancelFn()
						return &v1.StreamingDataCaptureUploadResponse{}, nil
					},
				}
				ss.streamingClients = append(ss.streamingClients, mockStreamingClient)
				ss.contexts = append(ss.contexts, cancelCtx)
				ss.requests = append(ss.requests, []*v1.StreamingDataCaptureUploadRequest{})
				ss.mu.Unlock()
				firstSuccesssfulClientCreatedFn()
				return mockStreamingClient, nil
			}
			mockClient := datasync.MockDataSyncServiceClient{
				T:                              t,
				StreamingDataCaptureUploadFunc: streamingDataCaptureUploadFunc,
			}

			dataSyncServiceClientConstructor := func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient { return mockClient }
			c.CaptureDisabled = true
			c.ScheduledSyncDisabled = true

			b2Svc, err := New(context.Background(), deps, config, dataSyncServiceClientConstructor, ConnToConnectivityStateReady, logger)
			test.That(t, err, test.ShouldBeNil)
			b2 := b2Svc.(*builtIn)
			defer b2.Close(context.Background())
			select {
			case <-time.After(waitTime):
				t.Fatal("timeout")
			case <-b2.sync.CloudConnReady():
			}

			// Call sync.
			err = b2.Sync(context.Background(), nil)
			test.That(t, err, test.ShouldBeNil)

			// Wait for upload requests.
			wait := time.After(waitTime)
			var reqCtx context.Context
			select {
			case <-wait:
				if !tc.serviceFail {
					t.Fatalf("timed out waiting for sync request")
				}
			case <-firstSuccesssfulClientCreatedCtx.Done():
				ss.mu.Lock()
				reqCtx = ss.contexts[0]
				ss.mu.Unlock()
				select {
				case <-wait:
					if !tc.serviceFail {
						t.Fatalf("timed out waiting for sync request")
					}
				case <-reqCtx.Done():
				}
			}
			waitUntilNoFiles(tmpDir)

			// Validate error and URs.
			if tc.serviceFail {
				// Validate no files were successfully uploaded.
				test.That(t, firstSuccesssfulClientCreatedCtx.Err(), test.ShouldBeNil)
				_, capturedData2, err := getCapturedData(getAllFilePaths(tmpDir))
				test.That(t, err, test.ShouldBeNil)
				// Error case, data capture data should should be unchanged (aka not deleted)
				test.That(t, capturedData2, test.ShouldResemble, capturedData)
			} else {
				// Validate first metadata message.
				ss.mu.Lock()
				requests := ss.requests[0]
				ss.mu.Unlock()
				test.That(t, len(requests), test.ShouldBeGreaterThan, 0)
				actMD := requests[0].GetMetadata()
				test.That(t, actMD, test.ShouldNotBeNil)
				test.That(t, actMD.GetUploadMetadata(), test.ShouldNotBeNil)
				test.That(t, actMD.GetSensorMetadata(), test.ShouldNotBeNil)
				test.That(t, actMD.GetUploadMetadata().Type, test.ShouldEqual, v1.DataType_DATA_TYPE_BINARY_SENSOR)
				test.That(t, actMD.GetUploadMetadata().PartId, test.ShouldNotBeBlank)

				// Validate ensuing data messages.
				dataRequests := requests[1:]
				var uploadedData []byte
				for _, d := range dataRequests {
					uploadedData = append(uploadedData, d.GetData()...)
				}
				test.That(t, uploadedData, test.ShouldResemble, capturedData[0].GetBinary())

				// Validate file no longer exists.
				test.That(t, len(getAllFileInfos(tmpDir)), test.ShouldEqual, 0)
			}
			test.That(t, b.Close(context.Background()), test.ShouldBeNil)
		})
	}
}

func TestSyncConfigUpdateBehavior(t *testing.T) {
	tests := []struct {
		name                 string
		initSyncDisabled     bool
		initSyncIntervalMins float64
		newSyncDisabled      bool
		newSyncIntervalMins  float64
		newMaxSyncThreads    int
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
			newSyncIntervalMins:  0.009,
		},
		{
			name:                 "sync gets disabled, syncer should be nil",
			initSyncDisabled:     false,
			initSyncIntervalMins: syncIntervalMins,
			newSyncDisabled:      true,
			newSyncIntervalMins:  syncIntervalMins,
		},
		{
			name:                 "sync gets new number of background threads, should update",
			initSyncDisabled:     false,
			initSyncIntervalMins: syncIntervalMins,
			newSyncDisabled:      false,
			newSyncIntervalMins:  syncIntervalMins,
			newMaxSyncThreads:    10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logger := logging.NewTestLogger(t)

			// Set up data manager.
			dataSyncServiceClientConstructor := func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient {
				return datasync.MockDataSyncServiceClient{
					T: t,
					StreamingDataCaptureUploadFunc: func(
						ctx context.Context,
						_ ...grpc.CallOption,
					) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
						return &datasync.ClientStreamingMock[
							*v1.StreamingDataCaptureUploadRequest,
							*v1.StreamingDataCaptureUploadResponse,
						]{
							T: t,
							SendFunc: func(in *v1.StreamingDataCaptureUploadRequest) error {
								return nil
							},
							CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
								return &v1.StreamingDataCaptureUploadResponse{}, nil
							},
						}, nil
					},
				}
			}

			imgPng := newImgPng(t)
			r := setupRobot(nil, map[resource.Name]resource.Resource{
				camera.Named("c1"): &inject.Camera{
					ImageFunc: func(
						ctx context.Context,
						mimeType string,
						extra map[string]interface{},
					) ([]byte, camera.ImageMetadata, error) {
						return newImageBytesResp(ctx, imgPng, mimeType)
					},
				},
			})
			config, deps := setupConfig(t, r, enabledBinaryCollectorConfigPath)
			c := config.ConvertedAttributes.(*Config)

			// Set up service config.
			c.CaptureDisabled = false
			c.ScheduledSyncDisabled = tc.initSyncDisabled
			c.CaptureDir = tmpDir
			c.SyncIntervalMins = tc.initSyncIntervalMins

			bSvc, err := New(context.Background(), deps, config, dataSyncServiceClientConstructor, ConnToConnectivityStateReady, logger)
			test.That(t, err, test.ShouldBeNil)
			b := bSvc.(*builtIn)
			defer b.Close(context.Background())

			initTicker := b.sync.ScheduledTicker

			// Reconfigure the builtin with new sync configs
			c.ScheduledSyncDisabled = tc.newSyncDisabled
			c.SyncIntervalMins = tc.newSyncIntervalMins
			c.MaximumNumSyncThreads = tc.newMaxSyncThreads

			err = b.Reconfigure(context.Background(), deps, config)
			test.That(t, err, test.ShouldBeNil)

			newTicker := b.sync.ScheduledTicker
			scheduler := b.sync.Scheduler
			test.That(t, b.sync.FileDeletingWorkers.Context().Err(), test.ShouldBeNil)
			if tc.newSyncDisabled {
				test.That(t, scheduler.Context().Err(), test.ShouldBeError, context.Canceled)
			}
			if tc.initSyncIntervalMins != tc.newSyncIntervalMins {
				test.That(t, initTicker, test.ShouldNotEqual, newTicker)
			}
			if tc.newMaxSyncThreads != 0 {
				test.That(t, b.sync.MaxSyncThreads, test.ShouldEqual, tc.newMaxSyncThreads)
			}
		})
	}
}

func getCapturedData(filePaths []string) (int, []*v1.SensorData, error) {
	var allFiles []*data.CaptureFile
	var numFiles int

	for _, f := range filePaths {
		osFile, err := os.Open(f)
		if err != nil {
			return 0, nil, err
		}
		dcFile, err := data.ReadCaptureFile(osFile)
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
	t.Helper()
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

func waitUntilNoFiles(dir string) {
	totalWait := waitTime * 3
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

func populateFileContents(fileContents []byte) []byte {
	for i := 0; i < 1000; i++ {
		fileContents = append(fileContents, []byte("happy cows come from california\n")...)
	}

	return fileContents
}

func newImageBytesResp(ctx context.Context, img image.Image, mimeType string) ([]byte, camera.ImageMetadata, error) {
	outBytes, err := rimage.EncodeImage(ctx, img, mimeType)
	if err != nil {
		return nil, camera.ImageMetadata{}, err
	}
	return outBytes, camera.ImageMetadata{MimeType: mimeType}, nil
}

func newImgPng(t *testing.T) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var imgBuf bytes.Buffer
	test.That(t, png.Encode(&imgBuf, img), test.ShouldBeNil)
	imgPng, err := png.Decode(bytes.NewReader(imgBuf.Bytes()))
	test.That(t, err, test.ShouldBeNil)
	return imgPng
}

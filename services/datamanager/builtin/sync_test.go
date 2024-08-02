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

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager/datacapture"
)

const (
	syncIntervalMins = 0.0008
	syncInterval     = time.Millisecond * 48
)

// TODO DATA-849: Add a test that validates that sync interval is accurately respected.
func TestSyncEnabled(t *testing.T) {
	// captureInterval := time.Millisecond * 10
	tests := []struct {
		name string
		// TODO: Flip disabled to be enabled
		syncStartDisabled bool
		syncEndDisabled   bool
	}{
		{
			name:              "config with sync disabled should sync nothing",
			syncStartDisabled: true,
			syncEndDisabled:   true,
		},
		{
			name:              "config with sync enabled should sync",
			syncStartDisabled: false,
			syncEndDisabled:   false,
		},
		{
			name:              "disabling sync should stop syncing",
			syncStartDisabled: false,
			syncEndDisabled:   true,
		},
		{
			name:              "enabling sync should trigger syncing to start",
			syncStartDisabled: true,
			syncEndDisabled:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := logging.NewTestLogger(t)
			// Set up server.
			// mockClock := clk.New()
			// Make mockClock the package level clock used by the builtin so that we can simulate time's passage
			// clock = mockClock
			tmpDir := t.TempDir()

			// Set up data manager.
			b, r := newBuiltIn(t)
			defer b.Close(context.Background())
			var secondConfigPropagated atomic.Bool
			firstCalledCtx, firstCalledFn := context.WithCancel(context.Background())
			secondCalledCtx, secondCalledFn := context.WithCancel(context.Background())
			mockClient := MockDataSyncServiceClient{
				DataCaptureUploadFunc: func(ctx context.Context, in *v1.DataCaptureUploadRequest, opts ...grpc.CallOption) (*v1.DataCaptureUploadResponse, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					firstCalledFn()
					if secondConfigPropagated.Load() {
						logger.Infof("second: %s", in.String())
						secondCalledFn()
					} else {
						logger.Infof("first: %s", in.String())
					}
					return &v1.DataCaptureUploadResponse{}, nil
				},
			}
			b.sync.DataSyncServiceClientConstructor = func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient {
				return mockClient
			}
			cfg, associations, deps := setupConfig(t, enabledBinaryCollectorConfigPath)

			// Set up service start config.
			cfg.CaptureDisabled = false
			cfg.ScheduledSyncDisabled = tc.syncStartDisabled
			cfg.CaptureDir = tmpDir
			cfg.SyncIntervalMins = syncIntervalMins

			resources := resourcesFromDeps(t, r, deps)
			t.Log("reconfiguring")
			err := b.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes:  cfg,
				AssociatedAttributes: associations,
			})
			test.That(t, err, test.ShouldBeNil)
			t.Log("waiting for config to propagate")
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				test.That(tb, b.sync.ConfigApplied(), test.ShouldBeTrue)
			})
			// t.Log("advancing clock by capture interval")
			// mockClock.Add(captureInterval)
			t.Log("waiting for data capture to write a data capture file")
			waitForCaptureFilesToExceedNFiles(tmpDir, 0, logger)
			t.Log("got a file")
			// t.Logf("advancing clock by sync interval: %s", syncInterval)
			// mockClock.Add(syncInterval)
			waitTime := time.Second * 2
			wait := time.After(waitTime)
			t.Logf("waiting up to %s for a file to be uploaded", waitTime)
			select {
			case <-wait:
			case <-firstCalledCtx.Done():
			}

			if tc.syncStartDisabled {
				test.That(t, firstCalledCtx.Err(), test.ShouldBeNil)
			} else {
				test.That(t, firstCalledCtx.Err(), test.ShouldBeError, context.Canceled)
			}

			// Set up service end config.
			cfg.ScheduledSyncDisabled = tc.syncEndDisabled

			resources = resourcesFromDeps(t, r, deps)
			err = b.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes:  cfg,
				AssociatedAttributes: associations,
			})
			test.That(t, err, test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				test.That(tb, b.sync.ConfigApplied(), test.ShouldBeTrue)
			})
			secondConfigPropagated.Store(true)

			// mockClock.Add(captureInterval)
			waitForCaptureFilesToExceedNFiles(tmpDir, 0, logger)
			// mockClock.Add(syncInterval)
			wait = time.After(time.Second * 2)
			select {
			case <-wait:
			case <-secondCalledCtx.Done():
			}
			err = b.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			if !tc.syncEndDisabled {
				test.That(t, secondCalledCtx.Err(), test.ShouldBeError, context.Canceled)
			} else {
				test.That(t, secondCalledCtx.Err(), test.ShouldBeNil)
			}
		})
	}
}

// TODO DATA-849: Test concurrent capture and sync more thoroughly.
func TestDataCaptureUploadIntegration(t *testing.T) {
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	// datasync.RetryExponentialFactor.Store(int32(1))
	// datasync.InitialWaitTimeMillis.Store(int32(20))

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
			t.Logf("tc: %#v", tc)
			// Set up server.
			// mockClock := clk.New()
			// clock = mockClock
			// t.Logf("clock: %p, mockClock: %p ", clock, mockClock)
			tmpDir := t.TempDir()

			// Set up data manager.
			b, r := newBuiltIn(t)
			defer b.Close(context.Background())
			var cfg *Config
			var associations map[resource.Name]resource.AssociatedConfig
			var deps []string
			captureInterval := time.Millisecond * 10
			if tc.emptyFile {
				cfg, associations, deps = setupConfig(t, infrequentCaptureTabularCollectorConfigPath)
			} else {
				if tc.dataType == v1.DataType_DATA_TYPE_TABULAR_SENSOR {
					cfg, associations, deps = setupConfig(t, enabledTabularCollectorConfigPath)
				} else {
					cfg, associations, deps = setupConfig(t, enabledBinaryCollectorConfigPath)
				}
			}

			// Set up service config with only capture enabled.
			cfg.CaptureDisabled = false
			cfg.ScheduledSyncDisabled = true
			cfg.SyncIntervalMins = syncIntervalMins
			cfg.CaptureDir = tmpDir

			resources := resourcesFromDeps(t, r, deps)
			err := b.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes:  cfg,
				AssociatedAttributes: associations,
			})
			test.That(t, err, test.ShouldBeNil)
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				test.That(tb, b.sync.ConfigApplied(), test.ShouldBeTrue)
			})

			// Let it capture a bit, then close.
			// for i := 0; i < 20; i++ {
			// 	t.Logf("mockNow: UnixMilli: %d", mockClock.Now().UnixMilli())
			// 	mockClock.Add(captureInterval)
			// }
			time.Sleep(captureInterval * 20)
			err = b.Close(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Get all captured data.
			numFiles, capturedData, err := getCapturedData(tmpDir)
			t.Logf("numFiles: %d", numFiles)
			test.That(t, err, test.ShouldBeNil)
			if tc.emptyFile {
				test.That(t, len(capturedData), test.ShouldEqual, 0)
			} else {
				test.That(t, len(capturedData), test.ShouldBeGreaterThan, 0)
			}

			// Turn builtin back on with capture disabled.
			b2, r := newBuiltIn(t)
			defer b2.Close(context.Background())
			var callCount atomic.Uint64
			var fail atomic.Bool
			failChan := make(chan *v1.DataCaptureUploadRequest, numFiles)
			successChan := make(chan *v1.DataCaptureUploadRequest, numFiles)

			f := func(
				ctx context.Context,
				in *v1.DataCaptureUploadRequest,
				opts ...grpc.CallOption,
			) (*v1.DataCaptureUploadResponse, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				defer func() { callCount.Add(1) }()
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
					opts ...grpc.CallOption,
				) (*v1.DataCaptureUploadResponse, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}

					defer func() { callCount.Add(1) }()
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

			mockClient := MockDataSyncServiceClient{
				DataCaptureUploadFunc: f,
			}
			b2.sync.DataSyncServiceClientConstructor = func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient {
				return mockClient
			}
			cfg.CaptureDisabled = true
			cfg.ScheduledSyncDisabled = tc.scheduledSyncDisabled
			cfg.SyncIntervalMins = syncIntervalMins
			resources = resourcesFromDeps(t, r, deps)
			t.Logf("Calling Reconfigure")
			err = b2.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes:  cfg,
				AssociatedAttributes: associations,
			})
			test.That(t, err, test.ShouldBeNil)
			t.Logf("waiting for b2.sync.ConfigPropagated")
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				test.That(tb, b2.sync.ConfigApplied(), test.ShouldBeTrue)
			})

			wait := time.After(time.Second * 30)
			if tc.failTransiently {
				failCount := 3
				for i := 0; i < failCount; i++ {
					t.Logf("waiting for %d files to fail", numFiles)
					for j := 0; j < numFiles; j++ {
						select {
						case <-wait:
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
				err = b2.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
				err = b2.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
			}

			var successfulReqs []*v1.DataCaptureUploadRequest
			// Get the successful requests
			t.Logf("syncInterval: %s, numFiles: %d", syncInterval, numFiles)
			for i := 0; i < numFiles; i++ {
				// mockClock.Add(syncInterval)
				select {
				case <-wait:
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
		})
	}
}

func TestArbitraryFileUpload(t *testing.T) {
	// Set exponential factor to 1 and retry wait time to 20ms so retries happen very quickly.
	// datasync.RetryExponentialFactor.Store(int32(1))
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
		{
			name:                 emptyFileTestName,
			manualSync:           false,
			scheduleSyncDisabled: false,
			serviceFail:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up server.
			// no mocked time
			// clock = clk.New()
			additionalPathsDir := t.TempDir()
			captureDir := t.TempDir()

			// Set up b config.
			b, r := newBuiltIn(t)
			defer b.Close(context.Background())

			rs := []*v1.FileUploadRequest{}
			closeCtx, closeFn := context.WithCancel(context.Background())
			var uploadCount atomic.Uint64
			mockClient := MockDataSyncServiceClient{
				T: t,
				FileUploadFunc: func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error) {
					return &DataSyncService_FileUploadClientMock{
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

			b.sync.DataSyncServiceClientConstructor = func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient { return mockClient }
			cfg, associations, deps := setupConfig(t, disabledTabularCollectorConfigPath)
			cfg.ScheduledSyncDisabled = tc.scheduleSyncDisabled
			cfg.SyncIntervalMins = syncIntervalMins
			cfg.AdditionalSyncPaths = []string{additionalPathsDir}
			cfg.CaptureDir = captureDir
			cfg.FileLastModifiedMillis = 1
			t.Logf("cfg.AdditionalSyncPaths: %s, cfg.CaptureDir: %s", additionalPathsDir, cfg.CaptureDir)

			// Start builtin.
			resources := resourcesFromDeps(t, r, deps)
			t.Log("calling Reconfigure")
			err := b.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes:  cfg,
				AssociatedAttributes: associations,
			})
			test.That(t, err, test.ShouldBeNil)
			t.Log("waiting for config to propagate")
			testutils.WaitForAssertion(t, func(tb testing.TB) {
				tb.Helper()
				test.That(tb, b.sync.ConfigApplied(), test.ShouldBeTrue)
			})

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
			time.Sleep(time.Millisecond * time.Duration(cfg.FileLastModifiedMillis) * 2)

			// Advance the clock syncInterval so it tries to sync the files.

			// Call manual sync.
			if tc.manualSync {
				t.Log("calling manual sync")
				err = b.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeNil)
			}

			// Wait for upload requests.
			// Get the successful requests
			wait := time.After(time.Second * 3)
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

// func TestStreamingDCUpload(t *testing.T) {
// 	// Set max unary file size to 1 byte, so it uses the streaming rpc. Reset the original
// 	// value such that other tests take the correct code paths.
// 	origSize := sync.MaxUnaryFileSize
// 	sync.MaxUnaryFileSize = 1
// 	defer func() {
// 		sync.MaxUnaryFileSize = origSize
// 	}()

// 	tests := []struct {
// 		name        string
// 		serviceFail bool
// 	}{
// 		{
// 			name: "A data capture file greater than MaxUnaryFileSize should be successfully uploaded" +
// 				"via the streaming rpc.",
// 			serviceFail: false,
// 		},
// 		{
// 			name:        "if an error response is received from the backend, local files should not be deleted",
// 			serviceFail: true,
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			logger := logging.NewTestLogger(t)
// 			// Set up server.
// 			mockClock := clk.NewMock()
// 			clock = mockClock
// 			tmpDir := t.TempDir()

// 			// Set up data manager.
// 			b, r := newBuiltIn(t)
// 			defer b.Close(context.Background())
// 			var cfg *Config
// 			var associations map[resource.Name]resource.AssociatedConfig
// 			var deps []string
// 			captureInterval := time.Millisecond * 10
// 			cfg, associations, deps = setupConfig(t, enabledBinaryCollectorConfigPath)

// 			// Set up service config with just capture enabled.
// 			cfg.CaptureDisabled = false
// 			cfg.ScheduledSyncDisabled = true
// 			cfg.SyncIntervalMins = syncIntervalMins
// 			cfg.CaptureDir = tmpDir

// 			resources := resourcesFromDeps(t, r, deps)
// 			err := b.Reconfigure(context.Background(), resources, resource.Config{
// 				ConvertedAttributes:  cfg,
// 				AssociatedAttributes: associations,
// 			})
// 			test.That(t, err, test.ShouldBeNil)
// 			testutils.WaitForAssertion(t, func(tb testing.TB) {
// 				tb.Helper()
// 				test.That(tb, b.sync.ConfigPropagated.Load(), test.ShouldBeTrue)
// 			})

// 			// Capture an image, then close.
// 			mockClock.Add(captureInterval)
// 			waitForCaptureFilesToExceedNFiles(tmpDir, 0, logger)
// 			err = b.Close(context.Background())
// 			test.That(t, err, test.ShouldBeNil)

// 			// Get all captured data.
// 			_, capturedData, err := getCapturedData(tmpDir)
// 			test.That(t, err, test.ShouldBeNil)

// 			// Turn builtin back on with capture disabled.
// 			b2, r := newBuiltIn(t)
// 			defer b2.Close(context.Background())
// 			f := atomic.Bool{}
// 			f.Store(tc.serviceFail)
// 			mockClient := MockDataSyncServiceClient{
// 				streamingDCUploads: make(chan *MockStreamingDCClient, 10),
// 				Fail:               &f,
// 			}
// 			b2.SetSyncerConstructor(GetTestSyncerConstructorMock(mockClient))
// 			cfg.CaptureDisabled = true
// 			cfg.ScheduledSyncDisabled = true
// 			resources = resourcesFromDeps(t, r, deps)
// 			err = b2.Reconfigure(context.Background(), resources, resource.Config{
// 				ConvertedAttributes:  cfg,
// 				AssociatedAttributes: associations,
// 			})
// 			test.That(t, err, test.ShouldBeNil)
// 			testutils.WaitForAssertion(t, func(tb testing.TB) {
// 				tb.Helper()
// 				test.That(tb, b2.sync.ConfigPropagated.Load(), test.ShouldBeTrue)
// 			})

// 			// Call sync.
// 			err = b2.Sync(context.Background(), nil)
// 			test.That(t, err, test.ShouldBeNil)

// 			// Wait for upload requests.
// 			var uploads []*MockStreamingDCClient
// 			var urs []*v1.StreamingDataCaptureUploadRequest
// 			// Get the successful requests
// 			wait := time.After(time.Second * 3)
// 			select {
// 			case <-wait:
// 				if !tc.serviceFail {
// 					t.Fatalf("timed out waiting for sync request")
// 				}
// 			case r := <-mockClient.streamingDCUploads:
// 				uploads = append(uploads, r)
// 				select {
// 				case <-wait:
// 					t.Fatalf("timed out waiting for sync request")
// 				case <-r.closed:
// 					urs = append(urs, r.reqs...)
// 				}
// 			}
// 			waitUntilNoFiles(tmpDir)

// 			// Validate error and URs.
// 			remainingFiles := getAllFilePaths(tmpDir)
// 			if tc.serviceFail {
// 				// Validate no files were successfully uploaded.
// 				test.That(t, len(uploads), test.ShouldEqual, 0)
// 				// Error case, file should not be deleted.
// 				test.That(t, len(remainingFiles), test.ShouldEqual, 1)
// 			} else {
// 				// Validate first metadata message.
// 				test.That(t, len(uploads), test.ShouldEqual, 1)
// 				test.That(t, len(urs), test.ShouldBeGreaterThan, 0)
// 				actMD := urs[0].GetMetadata()
// 				test.That(t, actMD, test.ShouldNotBeNil)
// 				test.That(t, actMD.GetUploadMetadata(), test.ShouldNotBeNil)
// 				test.That(t, actMD.GetSensorMetadata(), test.ShouldNotBeNil)
// 				test.That(t, actMD.GetUploadMetadata().Type, test.ShouldEqual, v1.DataType_DATA_TYPE_BINARY_SENSOR)
// 				test.That(t, actMD.GetUploadMetadata().PartId, test.ShouldNotBeBlank)

// 				// Validate ensuing data messages.
// 				dataRequests := urs[1:]
// 				var actData []byte
// 				for _, d := range dataRequests {
// 					actData = append(actData, d.GetData()...)
// 				}
// 				test.That(t, actData, test.ShouldResemble, capturedData[0].GetBinary())

// 				// Validate file no longer exists.
// 				test.That(t, len(getAllFileInfos(tmpDir)), test.ShouldEqual, 0)
// 			}
// 			test.That(t, b.Close(context.Background()), test.ShouldBeNil)
// 		})
// 	}
// }

// func TestSyncConfigUpdateBehavior(t *testing.T) {
// 	newSyncIntervalMins := 0.009
// 	tests := []struct {
// 		name                 string
// 		initSyncDisabled     bool
// 		initSyncIntervalMins float64
// 		newSyncDisabled      bool
// 		newSyncIntervalMins  float64
// 		newMaxSyncThreads    int
// 	}{
// 		{
// 			name:                 "all sync config stays the same, syncer should not cancel, ticker stays the same",
// 			initSyncDisabled:     false,
// 			initSyncIntervalMins: syncIntervalMins,
// 			newSyncDisabled:      false,
// 			newSyncIntervalMins:  syncIntervalMins,
// 		},
// 		{
// 			name:                 "sync config changes, new ticker should be created for sync",
// 			initSyncDisabled:     false,
// 			initSyncIntervalMins: syncIntervalMins,
// 			newSyncDisabled:      false,
// 			newSyncIntervalMins:  newSyncIntervalMins,
// 		},
// 		{
// 			name:                 "sync gets disabled, syncer should be nil",
// 			initSyncDisabled:     false,
// 			initSyncIntervalMins: syncIntervalMins,
// 			newSyncDisabled:      true,
// 			newSyncIntervalMins:  syncIntervalMins,
// 		},
// 		{
// 			name:                 "sync gets new number of background threads, should update",
// 			initSyncDisabled:     false,
// 			initSyncIntervalMins: syncIntervalMins,
// 			newSyncDisabled:      false,
// 			newSyncIntervalMins:  syncIntervalMins,
// 			newMaxSyncThreads:    10,
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			// Set up server.
// 			mockClock := clk.NewMock()
// 			// Make mockClock the package level clock used by the builtin so that we can simulate time's passage
// 			clock = mockClock
// 			tmpDir := t.TempDir()

// 			// Set up data manager.
// 			b, r := newBuiltIn(t)
// 			defer b.Close(context.Background())
// 			mockClient := MockDataSyncServiceClient{
// 				SuccesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
// 				FailedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
// 				Fail:                &atomic.Bool{},
// 			}
// 			b.SetSyncerConstructor(GetTestSyncerConstructorMock(mockClient))
// 			cfg, associations, deps := setupConfig(t, enabledBinaryCollectorConfigPath)

// 			// Set up service config.
// 			cfg.CaptureDisabled = false
// 			cfg.ScheduledSyncDisabled = tc.initSyncDisabled
// 			cfg.CaptureDir = tmpDir
// 			cfg.SyncIntervalMins = tc.initSyncIntervalMins

// 			resources := resourcesFromDeps(t, r, deps)
// 			err := b.Reconfigure(context.Background(), resources, resource.Config{
// 				ConvertedAttributes:  cfg,
// 				AssociatedAttributes: associations,
// 			})
// 			test.That(t, err, test.ShouldBeNil)

// 			testutils.WaitForAssertion(t, func(tb testing.TB) {
// 				tb.Helper()
// 				test.That(tb, b.sync.ConfigPropagated.Load(), test.ShouldBeTrue)
// 			})

// 			initTicker := b.sync.SyncTicker

// 			// Reconfigure the builtin with new sync configs
// 			cfg.ScheduledSyncDisabled = tc.newSyncDisabled
// 			cfg.SyncIntervalMins = tc.newSyncIntervalMins
// 			cfg.MaximumNumSyncThreads = tc.newMaxSyncThreads

// 			err = b.Reconfigure(context.Background(), resources, resource.Config{
// 				ConvertedAttributes:  cfg,
// 				AssociatedAttributes: associations,
// 			})
// 			test.That(t, err, test.ShouldBeNil)

// 			testutils.WaitForAssertion(t, func(tb testing.TB) {
// 				tb.Helper()
// 				test.That(tb, b.sync.ConfigPropagated.Load(), test.ShouldBeTrue)
// 			})
// 			newTicker := b.sync.SyncTicker
// 			newSyncer := b.sync.Syncer
// 			newFileDeletionBackgroundWorker := b.sync.FileDeletionBackgroundWorkers

// 			if tc.newSyncDisabled {
// 				test.That(t, newSyncer, test.ShouldBeNil)
// 			}
// 			test.That(t, newFileDeletionBackgroundWorker, test.ShouldNotBeNil)
// 			if tc.initSyncIntervalMins != tc.newSyncIntervalMins {
// 				test.That(t, initTicker, test.ShouldNotEqual, newTicker)
// 			}
// 			if tc.newMaxSyncThreads != 0 {
// 				test.That(t, b.sync.MaxSyncThreads, test.ShouldEqual, tc.newMaxSyncThreads)
// 			}
// 		})
// 	}
// }

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

func populateFileContents(fileContents []byte) []byte {
	for i := 0; i < 1000; i++ {
		fileContents = append(fileContents, []byte("happy cows come from california\n")...)
	}

	return fileContents
}

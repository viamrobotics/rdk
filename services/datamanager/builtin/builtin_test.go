package builtin

import (
	"cmp"
	"context"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin/capture"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

var (
	// Robot config which specifies data manager service.
	enabledTabularCollectorConfigPath           = "services/datamanager/data/fake_robot_with_data_manager.json"
	enabledTabularManyCollectorsConfigPath      = "services/datamanager/data/fake_robot_with_many_collectors_data_manager.json"
	disabledTabularCollectorConfigPath          = "services/datamanager/data/fake_robot_with_disabled_collector.json"
	enabledBinaryCollectorConfigPath            = "services/datamanager/data/robot_with_cam_capture.json"
	infrequentCaptureTabularCollectorConfigPath = "services/datamanager/data/fake_robot_with_infrequent_capture.json"
	remoteCollectorConfigPath                   = "services/datamanager/data/fake_robot_with_remote_and_data_manager.json"
	emptyFileBytesSize                          = 90 // a "rounded down" size of leading metadata message
)

type pathologicalAssociatedConfig struct{}

func (p *pathologicalAssociatedConfig) Equals(resource.AssociatedConfig) bool                   { return false }
func (p *pathologicalAssociatedConfig) UpdateResourceNames(func(n resource.Name) resource.Name) {}
func (p *pathologicalAssociatedConfig) Link(conf *resource.Config)                              {}

func TestCollectorRegistry(t *testing.T) {
	collectors := data.DumpRegisteredCollectors()
	test.That(t, len(collectors), test.ShouldEqual, 28)
	mds := slices.SortedFunc(maps.Keys(collectors), func(a, b data.MethodMetadata) int {
		return cmp.Compare(a.String(), b.String())
	})
	rdkComponent := resource.APIType{Namespace: resource.APINamespace("rdk"), Name: "component"}
	rdkService := resource.APIType{Namespace: resource.APINamespace("rdk"), Name: "service"}
	test.That(t, mds, test.ShouldResemble, []data.MethodMetadata{
		{API: resource.API{Type: rdkComponent, SubtypeName: "arm"}, MethodName: "EndPosition"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "arm"}, MethodName: "JointPositions"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "board"}, MethodName: "Analogs"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "board"}, MethodName: "Gpios"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "camera"}, MethodName: "GetImages"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "camera"}, MethodName: "NextPointCloud"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "camera"}, MethodName: "ReadImage"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "encoder"}, MethodName: "TicksCount"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "gantry"}, MethodName: "Lengths"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "gantry"}, MethodName: "Position"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "motor"}, MethodName: "IsPowered"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "motor"}, MethodName: "Position"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "movement_sensor"}, MethodName: "AngularVelocity"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "movement_sensor"}, MethodName: "CompassHeading"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "movement_sensor"}, MethodName: "LinearAcceleration"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "movement_sensor"}, MethodName: "LinearVelocity"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "movement_sensor"}, MethodName: "Orientation"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "movement_sensor"}, MethodName: "Position"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "movement_sensor"}, MethodName: "Readings"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "power_sensor"}, MethodName: "Current"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "power_sensor"}, MethodName: "Power"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "power_sensor"}, MethodName: "Readings"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "power_sensor"}, MethodName: "Voltage"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "sensor"}, MethodName: "Readings"},
		{API: resource.API{Type: rdkComponent, SubtypeName: "servo"}, MethodName: "Position"},
		{API: resource.API{Type: rdkService, SubtypeName: "slam"}, MethodName: "PointCloudMap"},
		{API: resource.API{Type: rdkService, SubtypeName: "slam"}, MethodName: "Position"},
		{API: resource.API{Type: rdkService, SubtypeName: "vision"}, MethodName: "CaptureAllFromCamera"},
	})
}

func TestNew(t *testing.T) {
	logger := logging.NewTestLogger(t)

	t.Run("returns an error if called with a resource.Config that can't be converted into a builtin.*Config", func(t *testing.T) {
		ctx := context.Background()
		mockDeps := mockDeps(nil, nil)
		_, err := New(ctx, mockDeps, resource.Config{}, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)

		expErr := errors.New("incorrect config type: NativeConfig expected *builtin.Config but got <nil>. " +
			"Make sure the config type registered to the resource matches the one passed into NativeConfig")
		test.That(t, err, test.ShouldBeError, expErr)
	})

	t.Run("when run in an untrusted environment", func(t *testing.T) {
		ctx, err := utils.WithTrustedEnvironment(context.Background(), false)
		test.That(t, err, test.ShouldBeNil)
		t.Run("returns successfully if config uses the default capture dir", func(t *testing.T) {
			mockDeps := mockDeps(nil, nil)
			c := &Config{}
			test.That(t, c.getCaptureDir(), test.ShouldResemble, viamCaptureDotDir)
			b, err := New(
				ctx,
				mockDeps,
				resource.Config{ConvertedAttributes: c},
				datasync.NoOpCloudClientConstructor,
				connToConnectivityStateError,
				logger,
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, b, test.ShouldNotBeNil)
			test.That(t, b.Close(context.Background()), test.ShouldBeNil)
		})

		t.Run("returns an error if booted in an untrusted environment with a non default capture_dir", func(t *testing.T) {
			config := resource.Config{ConvertedAttributes: &Config{CaptureDir: "/tmp/sth/else"}}
			_, err = New(ctx, nil, config, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)
			test.That(t, err, test.ShouldBeError, ErrCaptureDirectoryConfigurationDisabled)
		})
	})

	t.Run("returns an error if deps don't contain the internal cloud service", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(
			ctx,
			resource.Dependencies{},
			resource.Config{ConvertedAttributes: &Config{}}, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)
		errExp := errors.New("Resource missing from dependencies. " +
			"Resource: rdk-internal:service:cloud_connection/builtin")
		test.That(t, err, test.ShouldBeError, errExp)
	})

	t.Run("returns an error if any of the config.AssociatedAttributes "+
		"can't be converted into a *datamanager.AssociatedConfig", func(t *testing.T) {
		ctx := context.Background()
		aa := map[resource.Name]resource.AssociatedConfig{arm.Named("arm1"): &pathologicalAssociatedConfig{}}
		config := resource.Config{ConvertedAttributes: &Config{}, AssociatedAttributes: aa}
		deps := mockDeps(nil, resource.Dependencies{arm.Named("arm1"): &inject.Arm{}})
		_, err := New(ctx, deps, config, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)
		test.That(t, err, test.ShouldBeError, errors.New("expected *datamanager.AssociatedConfig but got *builtin.pathologicalAssociatedConfig"))
	})

	t.Run("otherwise returns a new builtin and no error", func(t *testing.T) {
		ctx := context.Background()
		r := setupRobot(nil, map[resource.Name]resource.Resource{
			arm.Named("arm1"): &inject.Arm{
				EndPositionFunc: func(
					ctx context.Context,
					extra map[string]interface{},
				) (spatialmath.Pose, error) {
					return spatialmath.NewZeroPose(), nil
				},
			},
		})

		config, deps := setupConfig(t, r, enabledTabularCollectorConfigPath)
		b, err := New(ctx, deps, config, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b, test.ShouldNotBeNil)
		test.That(t, b.Close(context.Background()), test.ShouldBeNil)
	})
}

func TestReconfigure(t *testing.T) {
	logger := logging.NewTestLogger(t)
	b, closeFunc := builtinWithEmptyConfig(t, logger)
	defer closeFunc()

	t.Run("returns an error if called with a resource.Config that can't be converted into a builtin.*Config", func(t *testing.T) {
		ctx := context.Background()
		err := b.Reconfigure(ctx, mockDeps(nil, nil), resource.Config{})
		expErr := errors.New("incorrect config type: NativeConfig expected *builtin.Config but got <nil>. " +
			"Make sure the config type registered to the resource matches the one passed into NativeConfig")
		test.That(t, err, test.ShouldBeError, expErr)
	})

	t.Run("when run in an untrusted environment", func(t *testing.T) {
		ctx, err := utils.WithTrustedEnvironment(context.Background(), false)
		test.That(t, err, test.ShouldBeNil)
		t.Run("returns successfully if config uses the default capture dir", func(t *testing.T) {
			mockDeps := mockDeps(nil, nil)
			c := &Config{}
			test.That(t, c.getCaptureDir(), test.ShouldResemble, viamCaptureDotDir)
			err := b.Reconfigure(ctx, mockDeps, resource.Config{ConvertedAttributes: c})
			test.That(t, err, test.ShouldBeNil)
		})

		t.Run("returns an error if booted in an untrusted environment with a non default capture_dir", func(t *testing.T) {
			config := resource.Config{ConvertedAttributes: &Config{CaptureDir: "/tmp/sth/else"}}
			err := b.Reconfigure(ctx, nil, config)
			test.That(t, err, test.ShouldBeError, ErrCaptureDirectoryConfigurationDisabled)
		})
	})

	t.Run("returns an error if deps don't contain the internal cloud service", func(t *testing.T) {
		ctx := context.Background()
		deps := resource.Dependencies{}
		config := resource.Config{ConvertedAttributes: &Config{}}
		err := b.Reconfigure(ctx, deps, config)
		errExp := errors.New("Resource missing from dependencies. Resource: " +
			"rdk-internal:service:cloud_connection/builtin")
		test.That(t, err, test.ShouldBeError, errExp)
	})

	t.Run("returns an error if any of the config.AssociatedAttributes can't"+
		" be converted into a *datamanager.AssociatedConfig", func(t *testing.T) {
		ctx := context.Background()
		aa := map[resource.Name]resource.AssociatedConfig{arm.Named("arm1"): &pathologicalAssociatedConfig{}}
		config := resource.Config{ConvertedAttributes: &Config{}, AssociatedAttributes: aa}
		deps := mockDeps(nil, resource.Dependencies{arm.Named("arm1"): &inject.Arm{}})
		err := b.Reconfigure(ctx, deps, config)
		test.That(t, err, test.ShouldBeError, errors.New("expected *datamanager.AssociatedConfig but got *builtin.pathologicalAssociatedConfig"))
	})

	t.Run("otherwise succeeds", func(t *testing.T) {
		ctx := context.Background()
		r := setupRobot(nil, map[resource.Name]resource.Resource{
			arm.Named("arm1"): &inject.Arm{
				EndPositionFunc: func(
					ctx context.Context,
					extra map[string]interface{},
				) (spatialmath.Pose, error) {
					return spatialmath.NewZeroPose(), nil
				},
			},
		})
		config, deps := setupConfig(t, r, enabledTabularCollectorConfigPath)
		err := b.Reconfigure(ctx, deps, config)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b, test.ShouldNotBeNil)
		test.That(t, b.Close(context.Background()), test.ShouldBeNil)
	})
}

func TestFileDeletion(t *testing.T) {
	logger := logging.NewTestLogger(t)
	mockClock := clock.NewMock()

	prevClock := clk
	clk = mockClock

	tempDir := t.TempDir()
	ctx := context.Background()

	fsThresholdToTriggerDeletion := datasync.FSThresholdToTriggerDeletion
	captureDirToFSUsageRatio := datasync.CaptureDirToFSUsageRatio
	t.Cleanup(func() {
		clk = prevClock
		datasync.FSThresholdToTriggerDeletion = fsThresholdToTriggerDeletion
		datasync.CaptureDirToFSUsageRatio = captureDirToFSUsageRatio
	})

	datasync.FSThresholdToTriggerDeletion = math.SmallestNonzeroFloat64
	datasync.CaptureDirToFSUsageRatio = math.SmallestNonzeroFloat64

	r := setupRobot(nil, map[resource.Name]resource.Resource{
		arm.Named("arm1"): &inject.Arm{
			EndPositionFunc: func(
				ctx context.Context,
				extra map[string]interface{},
			) (spatialmath.Pose, error) {
				return spatialmath.NewZeroPose(), nil
			},
			JointPositionsFunc: func(
				ctx context.Context,
				extra map[string]interface{},
			) ([]referenceframe.Input, error) {
				return referenceframe.FloatsToInputs([]float64{1.0, 2.0, 3.0, 4.0}), nil
			},
			ModelFrameFunc: func() referenceframe.Model {
				return nil
			},
		},
		gantry.Named("gantry1"): &inject.Gantry{
			PositionFunc: func(
				ctx context.Context,
				extra map[string]interface{},
			) ([]float64, error) {
				return []float64{1, 2, 3}, nil
			},
			LengthsFunc: func(
				ctx context.Context,
				extra map[string]interface{},
			) ([]float64, error) {
				return []float64{1, 2, 3}, nil
			},
		},
	})
	config, deps := setupConfig(t, r, enabledTabularManyCollectorsConfigPath)
	timeoutCtx, timeout := context.WithTimeout(ctx, time.Second*5)
	defer timeout()
	// create sync clock so we can control when a single iteration of file deltion happens
	c := config.ConvertedAttributes.(*Config)
	c.CaptureDir = tempDir
	// MaximumCaptureFileSizeBytes is set to 1 so that each reading becomes its own capture file
	// and we can confidently read the capture file without it's contents being modified by the collector
	c.MaximumCaptureFileSizeBytes = 1
	bSvc, err := New(ctx, deps, config, datasync.NoOpCloudClientConstructor, connToConnectivityStateError, logger)
	test.That(t, err, test.ShouldBeNil)
	b := bSvc.(*builtIn)
	defer b.Close(context.Background())
	// advance the clock so that the collectors can start
	mockClock.Add(data.GetDurationFromHz(100))

	// flush and close collectors to ensure we have exactly 4 files
	// close capture to stop it from writing more files
	b.capture.Close(ctx)

	// number of capture files is based on the number of unique
	// collectors in the robot config used in this test
	test.That(t, waitForCaptureFilesToEqualNFiles(timeoutCtx, tempDir, 4, logger), test.ShouldBeNil)

	filesInfos := getAllFilePaths(tempDir)
	test.That(t, len(filesInfos), test.ShouldEqual, 4)
	t.Log("before")
	for _, f := range filesInfos {
		t.Log(f)
	}
	// since we've written 4 files and hit the threshold, we expect
	// the first to be deleted
	expectedDeletedFile := filesInfos[0]

	// run forward by the CheckDeleteExcessFilesInterval to delete any files
	mockClock.Add(datasync.CheckDeleteExcessFilesInterval)
	test.That(t, waitForCaptureFilesToEqualNFiles(timeoutCtx, tempDir, 3, logger), test.ShouldBeNil)
	newFiles := getAllFilePaths(tempDir)

	t.Log("after")
	for _, fn := range newFiles {
		t.Log(fn)
	}
	test.That(t, len(newFiles), test.ShouldEqual, 3)
	test.That(t, newFiles, test.ShouldNotContain, expectedDeletedFile)
}

func TestSync(t *testing.T) {
	logger := logging.NewTestLogger(t)
	tests := []struct {
		name                 string
		dataType             v1.DataType
		failTransiently      bool
		connStateConstructor func(rpc.ClientConn) datasync.ConnectivityState
		cloudConnectionErr   error
	}{
		{
			name:                 "manual sync should return success and enqueue captured tabular data to be synced if got ready cloud connection",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			connStateConstructor: ConnToConnectivityStateReady,
			cloudConnectionErr:   nil,
		},
		{
			name:                 "manual sync should return success and enqueue captured binary data to be synced if got ready cloud connection",
			dataType:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			connStateConstructor: ConnToConnectivityStateReady,
			cloudConnectionErr:   nil,
		},
		{
			name: "manual sync should return success and enqueue captured tabular " +
				"data to be synced if transient errors are encountered",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			failTransiently:      true,
			connStateConstructor: ConnToConnectivityStateReady,
			cloudConnectionErr:   nil,
		},
		{
			name: "manual sync should return success and enqueue captured binary data to be " +
				"synced if transient errors are encountered",
			dataType:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			failTransiently:      true,
			connStateConstructor: ConnToConnectivityStateReady,
			cloudConnectionErr:   nil,
		},
		{
			name:               "manual sync should return an error and leave captured tabular data on disk if unable to get cloud connection",
			dataType:           v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name:               "manual sync should return an error and leave captured binary data on disk if unable to get cloud connection",
			dataType:           v1.DataType_DATA_TYPE_BINARY_SENSOR,
			cloudConnectionErr: errors.New("dial error"),
		},
		{
			name: "manual sync should return an success and leave captured tabular data on " +
				"disk if cloud connection was created but is currently not ready",
			dataType:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			connStateConstructor: connToConnectivityStateError,
			cloudConnectionErr:   nil,
		},
		{
			name: "manual sync should return an success and leave captured binary data on " +
				"disk if cloud connection was created but is currently not ready",
			dataType:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			connStateConstructor: connToConnectivityStateError,
			cloudConnectionErr:   nil,
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
						return spatialmath.NewZeroPose(), nil
					},
				},
			})
			config, deps = setupConfig(t, r, enabledTabularCollectorConfigPath)

			if tc.dataType == v1.DataType_DATA_TYPE_BINARY_SENSOR {
				r = setupRobot(tc.cloudConnectionErr, map[resource.Name]resource.Resource{
					camera.Named("c1"): &inject.Camera{
						ImageFunc: func(
							ctx context.Context,
							mimeType string,
							extra map[string]interface{},
						) ([]byte, camera.ImageMetadata, error) {
							outBytes, err := rimage.EncodeImage(ctx, imgPng, mimeType)
							if err != nil {
								return nil, camera.ImageMetadata{}, err
							}
							return outBytes, camera.ImageMetadata{MimeType: mimeType}, nil
						},
					},
				})
				config, deps = setupConfig(t, r, enabledBinaryCollectorConfigPath)
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
			test.That(t, len(capturedData), test.ShouldBeGreaterThan, 0)

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
				t.Log("WILL FAIL TRANSIENTLY")
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
						t.Log("FAIL!")
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
			c.ScheduledSyncDisabled = true
			c.SyncIntervalMins = syncIntervalMins

			b2Svc, err := New(context.Background(), deps, config, dataSyncServiceClientConstructor, tc.connStateConstructor, logger)
			test.That(t, err, test.ShouldBeNil)
			b2 := b2Svc.(*builtIn)

			if tc.cloudConnectionErr != nil {
				select {
				case <-time.After(time.Second):
				case <-b2.sync.CloudConnReady():
					t.Fatal("should not happen")
				}

				err = b2.Sync(context.Background(), nil)
				test.That(t, err, test.ShouldBeError, errors.New("not connected to the cloud"))
				err = b2.Close(context.Background())
				test.That(t, err, test.ShouldBeNil)

				// Validate that all previously captured data is still on disk
				_, capturedData2, err := getCapturedData(getAllFilePaths(tmpDir))
				test.That(t, err, test.ShouldBeNil)
				compareSensorData(t, tc.dataType, capturedData2, capturedData)
			} else {
				select {
				case <-time.After(time.Second):
					t.Fatal("timeout")
				case <-b2.sync.CloudConnReady():
				}
				waitTime := time.Second * 5
				timeoutCtx, timeoutFn := context.WithTimeout(context.Background(), waitTime)
				defer timeoutFn()
				w := goutils.NewStoppableWorkers(timeoutCtx)
				defer w.Stop()
				w.Add(func(ctx context.Context) { test.That(t, b2.Sync(ctx, nil), test.ShouldBeNil) })
				wait := time.After(waitTime)
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
				var successfulReqs []*v1.DataCaptureUploadRequest
				// Get the successful requests
				for i := 0; i < numFiles; i++ {
					select {
					case <-wait:
						if tc.cloudConnectionErr == nil {
							t.Fatalf("timed out waiting for sync request")
						}
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
			}
		})
	}
}

func TestSyncSensorFromDeps(t *testing.T) {
	logger := logging.NewTestLogger(t)
	emptyDeps := mockDeps(nil, nil)
	t.Run("returns (syncSensor=nil, syncSensorEnabled=false) if name is empty", func(t *testing.T) {
		syncSensor, syncSensorEnabled := syncSensorFromDeps("", emptyDeps, logger)
		test.That(t, syncSensor, test.ShouldBeNil)
		test.That(t, syncSensorEnabled, test.ShouldBeFalse)
	})

	t.Run("returns (syncSensor=nil, syncSensorEnabled=true) if name is not found in deps", func(t *testing.T) {
		syncSensor, syncSensorEnabled := syncSensorFromDeps("not in deps", emptyDeps, logger)
		test.That(t, syncSensor, test.ShouldBeNil)
		test.That(t, syncSensorEnabled, test.ShouldBeTrue)
	})

	t.Run("returns (syncSensor=non nil, syncSensorEnabled=true) if name is ound in deps", func(t *testing.T) {
		sensor1 := &inject.Sensor{}
		deps := mockDeps(nil, resource.Dependencies{
			sensor.Named("sensor1"): sensor1,
		})
		syncSensor, syncSensorEnabled := syncSensorFromDeps("sensor1", deps, logger)
		test.That(t, syncSensor, test.ShouldResemble, sensor1)
		test.That(t, syncSensorEnabled, test.ShouldBeTrue)
	})
}

func TestLookupCollectorConfigsByResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	t.Run("returns an empty capture.CollectorConfigsByResource and nil error when called with an empty config", func(t *testing.T) {
		emptyDeps := mockDeps(nil, nil)
		config := resource.Config{ConvertedAttributes: &Config{}}
		collectorConfigsByResource, err := lookupCollectorConfigsByResource(emptyDeps, config, "/some/capture/dir/path", logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(collectorConfigsByResource), test.ShouldEqual, 0)
	})

	armName := arm.Named("arm1")
	arm1 := &inject.Arm{}
	cameraName := arm.Named("camrea1")
	camera1 := &inject.Camera{}
	depsWithoutCamera := mockDeps(nil, resource.Dependencies{
		armName: arm1,
	})
	depsWithCamera := mockDeps(nil, resource.Dependencies{
		armName:    arm1,
		cameraName: camera1,
	})

	t.Run("returns error when AssociatedAttributes of resource is not of type *datamanager.AssociatedConfig", func(t *testing.T) {
		config := resource.Config{
			ConvertedAttributes:  &Config{},
			AssociatedAttributes: map[resource.Name]resource.AssociatedConfig{armName: nil},
		}
		_, err := lookupCollectorConfigsByResource(depsWithoutCamera, config, "/some/capture/dir/path", logger)
		test.That(t, err, test.ShouldBeError, errors.New("expected *datamanager.AssociatedConfig but got <nil>"))
	})

	t.Run("does not add resources from AssociatedAttributwes which don't exist in deps", func(t *testing.T) {
		armCaptureMethods := []datamanager.DataCaptureConfig{
			{
				Name:               armName,
				Method:             "JointPositions",
				CaptureFrequencyHz: 1.0,
				AdditionalParams:   map[string]string{"some": "params"},
			},
			{
				Name:               armName,
				Method:             "CurrentInputs",
				CaptureFrequencyHz: 2.0,
				AdditionalParams:   map[string]string{"some_other": "params"},
			},
		}
		config := resource.Config{
			ConvertedAttributes: &Config{},
			AssociatedAttributes: map[resource.Name]resource.AssociatedConfig{
				armName: &datamanager.AssociatedConfig{
					CaptureMethods: armCaptureMethods,
				},
				cameraName: &datamanager.AssociatedConfig{
					CaptureMethods: []datamanager.DataCaptureConfig{
						{
							Name:               cameraName,
							Method:             "NextPointCloud",
							CaptureFrequencyHz: 3.0,
							AdditionalParams:   map[string]string{"some additional": "params"},
						},
					},
				},
			},
		}
		captureDir := "/some/capture/dir/path"
		collectorConfigsByResource, err := lookupCollectorConfigsByResource(depsWithoutCamera, config, captureDir, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(collectorConfigsByResource), test.ShouldEqual, 1)
		updatedArmCaptureMethods := []datamanager.DataCaptureConfig{}
		for _, acm := range armCaptureMethods {
			acm.CaptureDirectory = captureDir
			updatedArmCaptureMethods = append(updatedArmCaptureMethods, acm)
		}
		test.That(t, collectorConfigsByResource, test.ShouldResemble, capture.CollectorConfigsByResource{arm1: updatedArmCaptureMethods})
	})

	t.Run("adds the capture directory to the collector configs returned of all resources in deps", func(t *testing.T) {
		armCaptureMethods := []datamanager.DataCaptureConfig{
			{
				Name:               armName,
				Method:             "JointPositions",
				CaptureFrequencyHz: 1.0,
				AdditionalParams:   map[string]string{"some": "params"},
			},
			{
				Name:               armName,
				Method:             "CurrentInputs",
				CaptureFrequencyHz: 2.0,
				AdditionalParams:   map[string]string{"some_other": "params"},
			},
		}
		cameraCaptureMethods := []datamanager.DataCaptureConfig{
			{
				Name:               cameraName,
				Method:             "NextPointCloud",
				CaptureFrequencyHz: 3.0,
				AdditionalParams:   map[string]string{"some additional": "params"},
			},
		}
		config := resource.Config{
			ConvertedAttributes: &Config{},
			AssociatedAttributes: map[resource.Name]resource.AssociatedConfig{
				armName: &datamanager.AssociatedConfig{
					CaptureMethods: armCaptureMethods,
				},
				cameraName: &datamanager.AssociatedConfig{
					CaptureMethods: cameraCaptureMethods,
				},
			},
		}
		captureDir := "/some/capture/dir/path"
		collectorConfigsByResource, err := lookupCollectorConfigsByResource(depsWithCamera, config, captureDir, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(collectorConfigsByResource), test.ShouldEqual, 2)
		updatedArmCaptureMethods := []datamanager.DataCaptureConfig{}
		for _, acm := range armCaptureMethods {
			acm.CaptureDirectory = captureDir
			updatedArmCaptureMethods = append(updatedArmCaptureMethods, acm)
		}
		updatedCamereaCaptureMethods := []datamanager.DataCaptureConfig{}
		for _, acm := range cameraCaptureMethods {
			acm.CaptureDirectory = captureDir
			updatedCamereaCaptureMethods = append(updatedCamereaCaptureMethods, acm)
		}
		test.That(t, collectorConfigsByResource, test.ShouldResemble, capture.CollectorConfigsByResource{
			arm1:    updatedArmCaptureMethods,
			camera1: updatedCamereaCaptureMethods,
		})
	})
}

func builtinWithEmptyConfig(t *testing.T, logger logging.Logger) (datamanager.Service, func()) {
	mockDeps := mockDeps(nil, nil)
	b, err := New(
		context.Background(),
		mockDeps,
		resource.Config{ConvertedAttributes: &Config{}},
		datasync.NoOpCloudClientConstructor,
		connToConnectivityStateError,
		logger,
	)
	test.That(t, err, test.ShouldBeNil)
	return b, func() { test.That(t, b.Close(context.Background()), test.ShouldBeNil) }
}

func mockDeps(acquireConnectionErr error, deps resource.Dependencies) resource.Dependencies {
	if deps == nil {
		deps = make(resource.Dependencies)
	}
	deps[cloud.InternalServiceName] = &cloudinject.CloudConnectionService{
		Named:                cloud.InternalServiceName.AsNamed(),
		Conn:                 NewNoOpClientConn(),
		AcquireConnectionErr: acquireConnectionErr,
	}
	return deps
}

func setupRobot(
	acquireConnectionErr error,
	deps resource.Dependencies,
) *inject.Robot {
	r := &inject.Robot{}
	r.MockResourcesFromMap(mockDeps(acquireConnectionErr, deps))
	return r
}

func setupConfig(t *testing.T, r *inject.Robot, configPath string) (resource.Config, resource.Dependencies) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(context.Background(), utils.ResolveFile(configPath), logger)
	test.That(t, err, test.ShouldBeNil)
	return resourceConfigAndDeps(t, cfg, r)
}

func resourceConfigAndDeps(t *testing.T, cfg *config.Config, r *inject.Robot) (resource.Config, resource.Dependencies) {
	t.Helper()
	var config *resource.Config
	deps := resource.Dependencies{}
	// datamanager config should be in the config, if not test is inavlif
	for _, cTmp := range cfg.Services {
		c := cTmp
		if c.API == datamanager.API {
			if config != nil {
				t.Fatal("there should only be one instance of data manager")
			}
			_, ok := c.ConvertedAttributes.(*Config)
			test.That(t, ok, test.ShouldBeTrue)
			for name, assocConf := range c.AssociatedAttributes {
				_, ok := assocConf.(*datamanager.AssociatedConfig)
				test.That(t, ok, test.ShouldBeTrue)
				res, err := r.ResourceByName(name)
				// if the config requires a resource which we have not set a mock for, fail the test
				test.That(t, errors.Wrap(err, name.String()), test.ShouldBeNil)
				deps[name] = res
			}
			config = &c
		}
	}
	test.That(t, config, test.ShouldNotBeNil)
	builtinConfig, ok := config.ConvertedAttributes.(*Config)
	test.That(t, ok, test.ShouldBeTrue)
	ds, err := builtinConfig.Validate("")
	test.That(t, err, test.ShouldBeNil)
	for _, d := range ds {
		resName, err := resource.NewFromString(d)
		test.That(t, err, test.ShouldBeNil)
		res, err := r.ResourceByName(resName)
		test.That(t, err, test.ShouldBeNil)
		deps[resName] = res
	}
	return *config, deps
}

func getAllFilePaths(dir string) []string {
	_, paths := getAllFiles(dir)
	return paths
}

func getAllFileInfos(dir string) []os.FileInfo {
	infos, _ := getAllFiles(dir)
	return infos
}

func getAllFiles(dir string) ([]os.FileInfo, []string) {
	var fileInfos []os.FileInfo
	var filePaths []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// ignore errors/unreadable files and directories
			//nolint:nilerr
			return nil
		}
		fileInfos = append(fileInfos, info)
		filePaths = append(filePaths, path)
		return nil
	})
	return fileInfos, filePaths
}

// waitForCaptureFilesToEqualNFiles returns once `captureDir` has exactly `n` files of at least
// `emptyFileBytesSize` bytes.
func waitForCaptureFilesToEqualNFiles(ctx context.Context, captureDir string, n int, logger logging.Logger) error {
	var diagnostics sync.Once
	start := time.Now()
	captureFiles := 0
	files := []fs.FileInfo{}
	i := 0
	for {
		if err := ctx.Err(); err != nil {
			fNames := []string{}
			for _, f := range files {
				fNames = append(fNames, f.Name())
			}
			logger.Errorf("target: %d, iterations: %d, captureFiles: %d, files: %v", n, i, captureFiles, fNames)
			return err
		}
		files = getAllFileInfos(captureDir)
		captureFiles = 0
		for idx := range files {
			if files[idx].Size() > int64(emptyFileBytesSize) && filepath.Ext(files[idx].Name()) == data.CompletedCaptureFileExt {
				// Every datamanager file has at least 90 bytes of metadata. Wait for that to be
				// observed before considering the file as "existing".
				captureFiles++
			}
		}

		if captureFiles == n {
			return nil
		}

		time.Sleep(10 * time.Millisecond)
		i++
		if time.Since(start) > 10*time.Second {
			diagnostics.Do(func() {
				logger.Infow("waitForCaptureFilesToEqualNFiles diagnostics after 10 seconds of waiting", "numFiles", len(files), "expectedFiles", n)
				for idx, file := range files {
					logger.Infow("File information", "idx", idx, "dir", captureDir, "name", file.Name(), "size", file.Size())
				}
			})
		}
	}
}

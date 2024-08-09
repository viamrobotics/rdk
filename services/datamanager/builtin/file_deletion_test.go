package builtin

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	v1 "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	enabledTabularManyCollectorsConfigPath = "services/datamanager/data/fake_robot_with_many_collectors_data_manager.json"
)

// waitForCaptureFilesToEqualNFiles returns once `captureDir` has exactly `n` files of at least
// `emptyFileBytesSize` bytes.
func waitForCaptureFilesToEqualNFiles(captureDir string, n int, logger logging.Logger) {
	var diagnostics sync.Once
	start := time.Now()
	for {
		files := getAllFileInfos(captureDir)
		nonEmptyFiles := 0
		for idx := range files {
			if files[idx].Size() > int64(emptyFileBytesSize) {
				// Every datamanager file has at least 90 bytes of metadata. Wait for that to be
				// observed before considering the file as "existing".
				nonEmptyFiles++
			}
		}

		if nonEmptyFiles == n {
			return
		}

		time.Sleep(10 * time.Millisecond)
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

func TestFileDeletion(t *testing.T) {
	logger := logging.NewTestLogger(t)
	tempDir := t.TempDir()
	ctx := context.Background()

	fsThresholdToTriggerDeletion := datasync.FSThresholdToTriggerDeletion
	captureDirToFSUsageRatio := datasync.CaptureDirToFSUsageRatio
	filesystemPollInterval := datasync.FilesystemPollInterval
	t.Cleanup(func() {
		datasync.FSThresholdToTriggerDeletion = fsThresholdToTriggerDeletion
		datasync.CaptureDirToFSUsageRatio = captureDirToFSUsageRatio
		datasync.FilesystemPollInterval = filesystemPollInterval
	})

	datasync.FilesystemPollInterval = time.Millisecond
	datasync.FSThresholdToTriggerDeletion = math.SmallestNonzeroFloat64
	datasync.CaptureDirToFSUsageRatio = math.SmallestNonzeroFloat64

	// Set up data manager.

	config, deps := setupConfig(t, getInjectedRobot(mockDeps(offlineConn, map[resource.Name]resource.Resource{
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
			) (*v1.JointPositions, error) {
				return &v1.JointPositions{Values: []float64{1.0, 2.0, 3.0, 4.0}}, nil
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
	})), enabledTabularManyCollectorsConfigPath)
	b, err := NewBuiltIn(ctx, deps, config, noOpCloudClientConstructor, logger)
	test.That(t, err, test.ShouldBeNil)
	defer b.Close(context.Background())
	// TODO: RETURN HERE

	t.Log("test1")
	// number of capture files is based on the number of unique
	// collectors in the robot config used in this test
	waitForCaptureFilesToEqualNFiles(tempDir, 4, logger)
	t.Fatal("test2")

	files := getAllFileInfos(tempDir)
	test.That(t, len(files), test.ShouldEqual, 4)
	// since we've written 4 files and hit the threshold, we expect
	// the first to be deleted
	expectedDeletedFile := files[0]

	// run forward 20ms to delete any files
	// mockClock.Add(sync.FilesystemPollInterval)
	t.Log("test3")
	waitForCaptureFilesToEqualNFiles(tempDir, 3, logger)
	t.Log("test4")
	newFiles := getAllFileInfos(tempDir)
	test.That(t, len(newFiles), test.ShouldEqual, 3)
	test.That(t, newFiles, test.ShouldNotContain, expectedDeletedFile)
}

// func get2ComponentInjectedRobot() *inject.Robot {
// 	r := &inject.Robot{}
// 	rs := map[resource.Name]resource.Resource{}

// 	rs[cloud.InternalServiceName] = &cloudinject.CloudConnectionService{
// 		Named: cloud.InternalServiceName.AsNamed(),
// 	}

// 	injectedArm := &inject.Arm{}
// 	injectedArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
// 		return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
// 	}
// 	injectedArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
// 		return &pb.JointPositions{
// 			Values: []float64{0},
// 		}, nil
// 	}
// 	rs[arm.Named("arm1")] = injectedArm

// 	injectedGantry := &inject.Gantry{}
// 	injectedGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
// 		return []float64{0}, nil
// 	}
// 	injectedGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
// 		return []float64{0}, nil
// 	}
// 	rs[gantry.Named("gantry1")] = injectedGantry

// 	r.MockResourcesFromMap(rs)
// 	return r
// }

// func newTestDataManagerWithMultipleComponents(t *testing.T) (*builtIn, robot.Robot) {
// 	t.Helper()
// 	dmCfg := &Config{
// 		// set capture disabled to avoid kicking off polling twice in test
// 		CaptureDisabled: true,
// 	}
// 	cfgService := resource.Config{
// 		API:                 datamanager.API,
// 		ConvertedAttributes: dmCfg,
// 	}
// 	logger := logging.NewTestLogger(t)

// 	// Create local robot with injected arm and remote.
// 	r := get2ComponentInjectedRobot()

// 	resources := resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})
// 	svc, err := NewBuiltIn(context.Background(), resources, cfgService, logger)
// 	if err != nil {
// 		t.Log(err)
// 		t.FailNow()
// 	}
// 	return svc.(*builtIn), r
// }

// func newBuiltin(t *testing.T, tempDir string) *builtIn {
// 	b, r := newTestDataManagerWithMultipleComponents(t)
// 	mockClient := MockDataSyncServiceClient{}
// 	b.sync.DataSyncServiceClientConstructor = func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient { return mockClient }

// 	cfg, associations, deps := setupConfig(t, enabledTabularManyCollectorsConfigPath)

// 	// Set up service config.
// 	cfg.CaptureDisabled = false
// 	cfg.ScheduledSyncDisabled = false
// 	cfg.CaptureDir = tempDir
// 	cfg.SyncIntervalMins = syncIntervalMins

// 	resources := resourcesFromDeps(t, r, deps)
// 	err := b.Reconfigure(context.Background(), resources, resource.Config{
// 		ConvertedAttributes:  cfg,
// 		AssociatedAttributes: associations,
// 	})
// 	test.That(t, err, test.ShouldBeNil)
// 	testutils.WaitForAssertion(t, func(tb testing.TB) {
// 		tb.Helper()
// 		test.That(tb, b.sync.ConfigApplied(), test.ShouldBeTrue)
// 	})
// 	return b
// }

// type MockDataSyncServiceClient struct {
// 	T                     *testing.T
// 	DataCaptureUploadFunc func(
// 		ctx context.Context,
// 		in *v1.DataCaptureUploadRequest,
// 		opts ...grpc.CallOption,
// 	) (*v1.DataCaptureUploadResponse, error)
// 	FileUploadFunc func(
// 		ctx context.Context,
// 		opts ...grpc.CallOption,
// 	) (v1.DataSyncService_FileUploadClient, error)
// 	StreamingDataCaptureUploadFunc func(
// 		ctx context.Context,
// 		opts ...grpc.CallOption,
// 	) (v1.DataSyncService_StreamingDataCaptureUploadClient, error)
// }

// func (c MockDataSyncServiceClient) DataCaptureUpload(
// 	ctx context.Context,
// 	in *v1.DataCaptureUploadRequest,
// 	opts ...grpc.CallOption,
// ) (*v1.DataCaptureUploadResponse, error) {
// 	if c.DataCaptureUploadFunc == nil {
// 		err := errors.New("DataCaptureUpload unimplemented")
// 		c.T.Log(err)
// 		c.T.FailNow()
// 		return nil, err
// 	}
// 	return c.DataCaptureUploadFunc(ctx, in, opts...)
// }

// func (c MockDataSyncServiceClient) FileUpload(
// 	ctx context.Context,
// 	opts ...grpc.CallOption,
// ) (v1.DataSyncService_FileUploadClient, error) {
// 	if c.FileUploadFunc == nil {
// 		err := errors.New("FileUpload unimplmented")
// 		c.T.Log(err)
// 		c.T.FailNow()
// 		return nil, err
// 	}
// 	return c.FileUploadFunc(ctx, opts...)
// }

// func (c MockDataSyncServiceClient) StreamingDataCaptureUpload(
// 	ctx context.Context,
// 	opts ...grpc.CallOption,
// ) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
// 	if c.StreamingDataCaptureUploadFunc == nil {
// 		err := errors.New("StreamingDataCaptureUpload unimplmented")
// 		c.T.Log(err)
// 		c.T.FailNow()
// 		return nil, errors.New("StreamingDataCaptureUpload unimplmented")
// 	}
// 	return c.StreamingDataCaptureUploadFunc(ctx, opts...)
// }

// type DataSyncServiceFileUploadClientMock struct {
// 	T                *testing.T
// 	SendFunc         func(*v1.FileUploadRequest) error
// 	CloseAndRecvFunc func() (*v1.FileUploadResponse, error)
// }

// func (m *DataSyncServiceFileUploadClientMock) Send(in *v1.FileUploadRequest) error {
// 	if m.SendFunc == nil {
// 		err := errors.New("Send unimplmented")
// 		m.T.Log(err)
// 		m.T.FailNow()
// 		return err
// 	}
// 	return m.SendFunc(in)
// }

// func (m *DataSyncServiceFileUploadClientMock) CloseAndRecv() (*v1.FileUploadResponse, error) {
// 	if m.CloseAndRecvFunc == nil {
// 		err := errors.New("CloseAndRecv unimplmented")
// 		m.T.Log(err)
// 		m.T.FailNow()
// 		return nil, err
// 	}
// 	return m.CloseAndRecvFunc()
// }

// func (m *DataSyncServiceFileUploadClientMock) Header() (metadata.MD, error) {
// 	err := errors.New("Header unimplmented")
// 	m.T.Log(err)
// 	m.T.FailNow()
// 	return nil, err
// }

// func (m *DataSyncServiceFileUploadClientMock) Trailer() metadata.MD {
// 	m.T.Log("Trailer unimplemented")
// 	m.T.FailNow()
// 	return metadata.MD{}
// }

// func (m *DataSyncServiceFileUploadClientMock) CloseSend() error {
// 	err := errors.New("CloseSend unimplmented")
// 	m.T.Log(err)
// 	m.T.FailNow()
// 	return err
// }

// func (m *DataSyncServiceFileUploadClientMock) Context() context.Context {
// 	m.T.Log("Context unimplmented")
// 	m.T.FailNow()
// 	return nil
// }

// func (m *DataSyncServiceFileUploadClientMock) SendMsg(any) error {
// 	err := errors.New("SendMsg unimplmented")
// 	m.T.Log(err)
// 	m.T.FailNow()
// 	return err
// }

// func (m *DataSyncServiceFileUploadClientMock) RecvMsg(any) error {
// 	err := errors.New("RecvMsg unimplmented")
// 	m.T.Log(err)
// 	m.T.FailNow()
// 	return err
// }

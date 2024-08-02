package builtin

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	v1 "go.viam.com/api/app/datasync/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin/sync"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	enabledTabularManyCollectorsConfigPath = "services/datamanager/data/fake_robot_with_many_collectors_data_manager.json"
)

func TestFilePolling(t *testing.T) {
	logger := logging.NewTestLogger(t)
	// mockClock := clk.NewMock()
	// Make mockClock the package level clock used by the builtin so that we can simulate time's passage
	// clock = mockClock
	tempDir := t.TempDir()

	deletionTicker := sync.DeletionTicker
	filesystemPollInterval := sync.FilesystemPollInterval
	fsThresholdToTriggerDeletion := sync.FSThresholdToTriggerDeletion
	captureDirToFSUsageRatio := sync.CaptureDirToFSUsageRatio
	t.Cleanup(func() {
		sync.DeletionTicker = deletionTicker
		sync.FilesystemPollInterval = filesystemPollInterval
		sync.FSThresholdToTriggerDeletion = fsThresholdToTriggerDeletion
		sync.CaptureDirToFSUsageRatio = captureDirToFSUsageRatio
	})

	// sync.DeletionTicker = mockClock
	sync.FilesystemPollInterval = time.Millisecond * 20
	sync.FSThresholdToTriggerDeletion = math.SmallestNonzeroFloat64
	sync.CaptureDirToFSUsageRatio = math.SmallestNonzeroFloat64

	// Set up data manager.
	b := newBuiltin(t, tempDir)
	defer b.Close(context.Background())

	// run forward 10ms to capture 4 files then close the collectors,
	// mockClock.Add(captureInterval)
	capture := b.capture
	// flush and close collectors to ensure we have exactly 4 files
	capture.FlushCollectors()
	capture.CloseCollectors()
	// number of capture files is based on the number of unique
	// collectors in the robot config used in this test
	waitForCaptureFilesToEqualNFiles(tempDir, 4, logger)

	files := getAllFileInfos(tempDir)
	test.That(t, len(files), test.ShouldEqual, 4)
	// since we've written 4 files and hit the threshold, we expect
	// the first to be deleted
	expectedDeletedFile := files[0]

	// run forward 20ms to delete any files
	// mockClock.Add(sync.FilesystemPollInterval)
	waitForCaptureFilesToEqualNFiles(tempDir, 3, logger)
	newFiles := getAllFileInfos(tempDir)
	test.That(t, len(newFiles), test.ShouldEqual, 3)
	test.That(t, newFiles, test.ShouldNotContain, expectedDeletedFile)
}

func get2ComponentInjectedRobot() *inject.Robot {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}

	rs[cloud.InternalServiceName] = &cloudinject.CloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
	}

	injectedArm := &inject.Arm{}
	injectedArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
	}
	injectedArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
		return &pb.JointPositions{
			Values: []float64{0},
		}, nil
	}
	rs[arm.Named("arm1")] = injectedArm

	injectedGantry := &inject.Gantry{}
	injectedGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{0}, nil
	}
	injectedGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return []float64{0}, nil
	}
	rs[gantry.Named("gantry1")] = injectedGantry

	r.MockResourcesFromMap(rs)
	return r
}

func newTestDataManagerWithMultipleComponents(t *testing.T) (*builtIn, robot.Robot) {
	t.Helper()
	dmCfg := &Config{
		// set capture disabled to avoid kicking off polling twice in test
		CaptureDisabled: true,
	}
	cfgService := resource.Config{
		API:                 datamanager.API,
		ConvertedAttributes: dmCfg,
	}
	logger := logging.NewTestLogger(t)

	// Create local robot with injected arm and remote.
	r := get2ComponentInjectedRobot()

	resources := resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})
	svc, err := NewBuiltIn(context.Background(), resources, cfgService, logger)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return svc.(*builtIn), r
}

func newBuiltin(t *testing.T, tempDir string) *builtIn {
	b, r := newTestDataManagerWithMultipleComponents(t)
	mockClient := MockDataSyncServiceClient{}
	b.sync.DataSyncServiceClientConstructor = func(cc grpc.ClientConnInterface) v1.DataSyncServiceClient { return mockClient }

	cfg, associations, deps := setupConfig(t, enabledTabularManyCollectorsConfigPath)

	// Set up service config.
	cfg.CaptureDisabled = false
	cfg.ScheduledSyncDisabled = false
	cfg.CaptureDir = tempDir
	cfg.SyncIntervalMins = syncIntervalMins

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
	return b
}

type MockDataSyncServiceClient struct {
	T                              *testing.T
	DataCaptureUploadFunc          func(ctx context.Context, in *v1.DataCaptureUploadRequest, opts ...grpc.CallOption) (*v1.DataCaptureUploadResponse, error)
	FileUploadFunc                 func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error)
	StreamingDataCaptureUploadFunc func(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_StreamingDataCaptureUploadClient, error)
}

func (c MockDataSyncServiceClient) DataCaptureUpload(ctx context.Context, in *v1.DataCaptureUploadRequest, opts ...grpc.CallOption) (*v1.DataCaptureUploadResponse, error) {
	if c.DataCaptureUploadFunc == nil {
		err := errors.New("DataCaptureUpload unimplemented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, err
	}
	return c.DataCaptureUploadFunc(ctx, in, opts...)
}

func (c MockDataSyncServiceClient) FileUpload(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_FileUploadClient, error) {
	if c.FileUploadFunc == nil {
		err := errors.New("FileUpload unimplmented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, err
	}
	return c.FileUploadFunc(ctx, opts...)
}

func (c MockDataSyncServiceClient) StreamingDataCaptureUpload(ctx context.Context, opts ...grpc.CallOption) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
	if c.StreamingDataCaptureUploadFunc == nil {
		err := errors.New("StreamingDataCaptureUpload unimplmented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, errors.New("StreamingDataCaptureUpload unimplmented")
	}
	return c.StreamingDataCaptureUploadFunc(ctx, opts...)
}

type DataSyncService_FileUploadClientMock struct {
	T                *testing.T
	SendFunc         func(*v1.FileUploadRequest) error
	CloseAndRecvFunc func() (*v1.FileUploadResponse, error)
}

func (m *DataSyncService_FileUploadClientMock) Send(in *v1.FileUploadRequest) error {
	if m.SendFunc == nil {
		err := errors.New("Send unimplmented")
		m.T.Log(err)
		m.T.FailNow()
		return err
	}
	return m.SendFunc(in)
}

func (m *DataSyncService_FileUploadClientMock) CloseAndRecv() (*v1.FileUploadResponse, error) {
	if m.CloseAndRecvFunc == nil {
		err := errors.New("CloseAndRecv unimplmented")
		m.T.Log(err)
		m.T.FailNow()
		return nil, err
	}
	return m.CloseAndRecvFunc()
}

func (m *DataSyncService_FileUploadClientMock) Header() (metadata.MD, error) {
	err := errors.New("Header unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return nil, err
}

func (m *DataSyncService_FileUploadClientMock) Trailer() metadata.MD {
	m.T.Log("Trailer unimplemented")
	m.T.FailNow()
	return metadata.MD{}
}

func (m *DataSyncService_FileUploadClientMock) CloseSend() error {
	err := errors.New("CloseSend unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return err
}

func (m *DataSyncService_FileUploadClientMock) Context() context.Context {
	m.T.Log("Context unimplmented")
	m.T.FailNow()
	return nil
}

func (m *DataSyncService_FileUploadClientMock) SendMsg(any) error {
	err := errors.New("SendMsg unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return err
}

func (m *DataSyncService_FileUploadClientMock) RecvMsg(any) error {
	err := errors.New("RecvMsg unimplmented")
	m.T.Log(err)
	m.T.FailNow()
	return err
}

package builtin

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/golang/geo/r3"
	v1 "go.viam.com/api/app/datasync/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datasync"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	enabledTabularManyCollectorsConfigPath = "services/datamanager/data/fake_robot_with_many_collectors_data_manager.json"
)

func TestFileDeletionUsageCheck(t *testing.T) {
	tests := []struct {
		name              string
		deletionExpected  bool
		triggerThreshold  float64
		captureUsageRatio float64
		captureDirExists  bool
	}{
		{
			name:              "we should return false from deletion check if not at file system capacity threshold",
			deletionExpected:  false,
			triggerThreshold:  .99,
			captureUsageRatio: .99,
			captureDirExists:  true,
		},
		{
			name:              "we return true from deletion check if at file system capacity threshold",
			deletionExpected:  true,
			triggerThreshold:  math.SmallestNonzeroFloat64,
			captureUsageRatio: math.SmallestNonzeroFloat64,
			captureDirExists:  true,
		},
		{
			name: "we return false from deletion check" +
				"if at file system capacity threshold but not capture dir threshold",
			deletionExpected:  false,
			triggerThreshold:  math.SmallestNonzeroFloat64,
			captureUsageRatio: 1.0,
			captureDirExists:  true,
		},
		{
			name:              "we should return false from deletion check if capture dir does not exist",
			deletionExpected:  false,
			triggerThreshold:  .95,
			captureUsageRatio: .5,
			captureDirExists:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var tempCaptureDir string
			if tc.captureDirExists {
				tempCaptureDir = t.TempDir()
				// write testing files
				writeFiles(t, tempCaptureDir, []string{"1.capture", "2.capture"})
			}
			// overwrite thresholds
			fsThresholdToTriggerDeletion = tc.triggerThreshold
			captureDirToFSUsageRatio = tc.captureUsageRatio
			logger := logging.NewTestLogger(t)
			willDelete, err := shouldDeleteBasedOnDiskUsage(context.Background(), tempCaptureDir, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, willDelete, test.ShouldEqual, tc.deletionExpected)
		})
	}
}

func TestFileDeletion(t *testing.T) {
	tests := []struct {
		name                    string
		syncEnabled             bool
		shouldCancelContext     bool
		expectedDeleteFilenames []string
		fileList                []string
		syncerInProgressFiles   []string
	}{
		{
			name:                    "if sync disabled, file deleter should delete every 5th file",
			fileList:                []string{"0shouldDelete.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5shouldDelete.capture"},
			expectedDeleteFilenames: []string{"0shouldDelete.capture", "5shouldDelete.capture"},
		},
		{
			name:                    "if sync enabled and all files marked as in progress, file deleter should not delete any files",
			syncEnabled:             true,
			fileList:                []string{"0.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5.capture"},
			syncerInProgressFiles:   []string{"0.capture", "1.capture", "2.capture", "3.capture", "4.capture", "5.capture"},
			expectedDeleteFilenames: []string{},
		},
		{
			name:                    "if sync enabled and some files marked as inprogress, file deleter should delete less files",
			syncEnabled:             true,
			fileList:                []string{"0.capture", "1.capture", "2shouldDelete.capture", "3.capture", "4.capture", "5.capture"},
			syncerInProgressFiles:   []string{"0.capture", "1.capture"},
			expectedDeleteFilenames: []string{"2shouldDelete.capture"},
		},
		{
			name:                    "if sync disabled and files are still being written to, file deleter should not delete any files",
			fileList:                []string{"0.prog", "1.prog", "2.prog", "3.prog", "4.prog", "5.prog"},
			expectedDeleteFilenames: []string{},
		},
		{
			name:                    "file deleter should not delete non datacapture files",
			fileList:                []string{"0.fe", "1.fi", "2.fo", "3.fum", "4.foo", "5.capture"},
			expectedDeleteFilenames: []string{"5.capture"},
		},
		{
			name:                "if cancelled context is cancelled, file deleter should return an error",
			shouldCancelContext: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempCaptureDir := t.TempDir()
			logger := logging.NewTestLogger(t)
			mockClient := mockDataSyncServiceClient{
				succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
				failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
				fail:                &atomic.Bool{},
			}

			var syncer datasync.Manager
			if tc.syncEnabled {
				filesToSync := make(chan string)
				defer close(filesToSync)
				syncer = datasync.NewManager("rick astley", mockClient, logger, tempCaptureDir, datasync.MaxParallelSyncRoutines, filesToSync)
				defer syncer.Close()
			}

			filepaths := writeFiles(t, tempCaptureDir, tc.fileList)
			for _, file := range tc.syncerInProgressFiles {
				syncer.MarkInProgress(filepaths[file])
			}

			ctx, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()
			if tc.shouldCancelContext {
				cancelFunc()
			}
			deletedFileCount, err := deleteFiles(ctx, syncer, defaultDeleteEveryNth, tempCaptureDir, logger)
			if tc.shouldCancelContext {
				test.That(t, err, test.ShouldBeError, context.Canceled)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, deletedFileCount, test.ShouldEqual, len(tc.expectedDeleteFilenames))
				// get list of all files still in capture dir after deletion
				files := getFiles(t, tempCaptureDir)
				for _, deletedFile := range tc.expectedDeleteFilenames {
					test.That(t, files, test.ShouldNotContain, deletedFile)
				}
			}
		})
	}
}

func writeFiles(t *testing.T, dir string, filenames []string) map[string]string {
	t.Helper()
	fileContents := []byte("never gonna let you down")
	filePaths := map[string]string{}
	for _, filename := range filenames {
		filePath := fmt.Sprintf("%s/%s", dir, filename)
		err := os.WriteFile(filePath, fileContents, 0o755)
		test.That(t, err, test.ShouldBeNil)
		filePaths[filename] = filePath
	}
	return filePaths
}

func getFiles(t *testing.T, path string) []string {
	t.Helper()
	dir, err := os.Open(path)
	test.That(t, err, test.ShouldBeNil)
	defer dir.Close()
	files, err := dir.Readdir(-1)
	test.That(t, err, test.ShouldBeNil)
	output := []string{}
	for _, file := range files {
		output = append(output, file.Name())
	}
	return output
}

func TestFilePolling(t *testing.T) {
	logger := logging.NewTestLogger(t)
	mockClock := clk.NewMock()
	// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
	clock = mockClock
	deletionTicker = mockClock
	filesystemPollInterval = time.Millisecond * 20

	tempDir := t.TempDir()
	fsThresholdToTriggerDeletion = math.SmallestNonzeroFloat64
	captureDirToFSUsageRatio = math.SmallestNonzeroFloat64

	// Set up data manager.
	dmsvc, _ := newDMSvc(t, tempDir)
	defer dmsvc.Close(context.Background())

	// run forward 10ms to capture 4 files then close the collectors,
	mockClock.Add(captureInterval)
	flusher, ok := dmsvc.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)
	// flush and close collectors to ensure we have exactly 4 files
	flusher.flushCollectors()
	flusher.closeCollectors()
	// number of capture files is based on the number of unique
	// collectors in the robot config used in this test
	waitForCaptureFilesToEqualNFiles(tempDir, 4, logger)

	files := getAllFileInfos(tempDir)
	test.That(t, len(files), test.ShouldEqual, 4)
	// since we've written 4 files and hit the threshold, we expect
	// the first to be deleted
	expectedDeletedFile := files[0]

	// run forward 20ms to delete any files
	mockClock.Add(filesystemPollInterval)
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

func newTestDataManagerWithMultipleComponents(t *testing.T) (internal.DMService, robot.Robot) {
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
	return svc.(internal.DMService), r
}

func newDMSvc(t *testing.T, tempDir string) (internal.DMService, mockDataSyncServiceClient) {
	dmsvc, r := newTestDataManagerWithMultipleComponents(t)
	mockClient := mockDataSyncServiceClient{
		succesfulDCRequests: make(chan *v1.DataCaptureUploadRequest, 100),
		failedDCRequests:    make(chan *v1.DataCaptureUploadRequest, 100),
		fail:                &atomic.Bool{},
	}
	dmsvc.SetSyncerConstructor(getTestSyncerConstructorMock(mockClient))

	cfg, associations, deps := setupConfig(t, enabledTabularManyCollectorsConfigPath)

	// Set up service config.
	cfg.CaptureDisabled = false
	cfg.ScheduledSyncDisabled = false
	cfg.CaptureDir = tempDir
	cfg.SyncIntervalMins = syncIntervalMins

	resources := resourcesFromDeps(t, r, deps)
	err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
		ConvertedAttributes:  cfg,
		AssociatedAttributes: associations,
	})
	test.That(t, err, test.ShouldBeNil)
	b := dmsvc.(*builtIn)
	test.That(t, b.propagateDataSyncConfig(), test.ShouldBeNil)
	return dmsvc, mockClient
}

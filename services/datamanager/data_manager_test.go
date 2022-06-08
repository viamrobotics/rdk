package datamanager

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func (svc *Service) hasActiveCollectors() bool {
	for _, collector := range svc.collectors {
		if collector.Collector.IsCollecting() {
			return true
		}
	}
	return false
}

func TestNewDataManager(t *testing.T) {
	cfg := &Config{
		CaptureDir: "/path/to/capture",
	}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: cfg,
	}

	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	dataManager, err := New(context.Background(), r, cfgService, logger)
	svc := dataManager.(*Service)
	svc.Close(context.Background())

	test.That(t, err, test.ShouldBeNil)
}

func TestDiskUsage(t *testing.T) {
	// Test 50% disk usage.
	blocks := 3
	bSize := 4
	bAvail := 1
	bFree := 2
	mockStatfsFn := getMockStatfsFn(blocks, bSize, bAvail, bFree)

	du, err := DiskUsage(mockStatfsFn)
	test.That(t, err, test.ShouldBeNil)

	expectedAll := uint64(12)
	expectedUsed := uint64(4)
	expectedFree := uint64(8)
	expectedAvail := uint64(4)
	expectedPercentUsed := 50
	test.That(t, du, test.ShouldResemble,
		DiskStatus{expectedAll, expectedUsed, expectedFree, expectedAvail, expectedPercentUsed})

	// Test 33% disk usage.
	bAvail = 2
	bFree = 2
	mockStatfsFn = getMockStatfsFn(blocks, bSize, bAvail, bFree)

	du, err = DiskUsage(mockStatfsFn)
	test.That(t, err, test.ShouldBeNil)

	expectedUsed = 4
	expectedFree = 8
	expectedAvail = 8
	expectedPercentUsed = 33
	test.That(t, du, test.ShouldResemble,
		DiskStatus{expectedAll, expectedUsed, expectedFree, expectedAvail, expectedPercentUsed})

	// Test 0% disk usage.
	bAvail = 3
	bFree = 3
	mockStatfsFn = getMockStatfsFn(blocks, bSize, bAvail, bFree)

	du, err = DiskUsage(mockStatfsFn)
	test.That(t, err, test.ShouldBeNil)

	expectedUsed = 0
	expectedFree = 12
	expectedAvail = 12
	expectedPercentUsed = 0
	test.That(t, du, test.ShouldResemble,
		DiskStatus{expectedAll, expectedUsed, expectedFree, expectedAvail, expectedPercentUsed})
}

func TestRunStorageCheckWithDisabledAutoDeletion(t *testing.T) {
	// Set up a robot with an arm.
	tempDir, _ := ioutil.TempDir("", "whatever")
	cfg := &Config{
		CaptureDir: tempDir,
	}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: cfg,
	}
	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	const arm1Key = "arm1"
	arm1 := &inject.Arm{}
	arm1.GetEndPositionFunc = func(ctx context.Context) (*commonpb.Pose, error) {
		return &commonpb.Pose{X: 1, Y: 2, Z: 3}, nil
	}
	rs := map[resource.Name]interface{}{arm.Named(arm1Key): arm1}
	r.MockResourcesFromMap(rs)

	dataManager, err := New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	svc := dataManager.(*Service)

	// Mock 0% disk usage in the system.
	blocks := 3
	bSize := 4
	bAvail := 3
	bFree := 3
	svc.lock.Lock()
	svc.statfs = getMockStatfsFn(blocks, bSize, bAvail, bFree)
	svc.lock.Unlock()

	// Replace the default storage check with a more frequent one for testing purposes.
	sleepTime := time.Millisecond * 5
	svc.storageCheckCancelFn()
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	svc.storageCheckCancelFn = cancelFn
	time.Sleep(sleepTime)
	svc.runStorageCheckAndUpdateCollectors(cancelCtx, time.Millisecond)

	// Load config with an arm, maximum storage percent of 40%, and disabled auto-delete.
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/fake_robot_with_data_manager.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	svc.Update(context.Background(), conf)
	time.Sleep(sleepTime)

	// Check that collectors are running.
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeTrue)
	test.That(t, svc.maxStoragePercent, test.ShouldEqual, 40)
	test.That(t, svc.enableAutoDelete, test.ShouldBeFalse)

	// Mock 66% disk usage in the system.
	bAvail = 1
	bFree = 1
	svc.lock.Lock()
	svc.statfs = getMockStatfsFn(blocks, bSize, bAvail, bFree)
	svc.lock.Unlock()

	// The running storage check should close any active collectors since we have exceeded
	// the disk usage limit.
	time.Sleep(sleepTime)
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeFalse)

	// Mock 33% disk usage in the system.
	bAvail = 2
	bFree = 2
	svc.lock.Lock()
	svc.statfs = getMockStatfsFn(blocks, bSize, bAvail, bFree)
	svc.lock.Unlock()

	// The running storage check should reactivate collectors since we are now back below
	// the disk usage limit.
	time.Sleep(sleepTime)
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeTrue)

	svc.Close(context.Background())
}

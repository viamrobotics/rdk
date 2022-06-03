//go:build !linux
// +build !linux

package datamanager

import (
	"context"
	"io/ioutil"
	"syscall"
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

func getMockStatfsFn(blocks uint64, bsize uint32, bavail uint64, bfree uint64) func(string, *syscall.Statfs_t) error {
	return func(path string, stat *syscall.Statfs_t) error {
		stat.Blocks = blocks
		stat.Bsize = bsize
		stat.Bavail = bavail
		stat.Bfree = bfree
		return nil
	}
}

func TestDiskUsage(t *testing.T) {
	// Test 50% disk usage.
	blocks := uint64(3)
	bSize := uint32(4)
	bAvail := uint64(1)
	bFree := uint64(2)
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
	bAvail = uint64(2)
	bFree = uint64(2)
	mockStatfsFn = getMockStatfsFn(blocks, bSize, bAvail, bFree)

	du, err = DiskUsage(mockStatfsFn)
	test.That(t, err, test.ShouldBeNil)

	expectedUsed = uint64(4)
	expectedFree = uint64(8)
	expectedAvail = uint64(8)
	expectedPercentUsed = 33
	test.That(t, du, test.ShouldResemble,
		DiskStatus{expectedAll, expectedUsed, expectedFree, expectedAvail, expectedPercentUsed})

	// Test 0% disk usage.
	bAvail = uint64(3)
	bFree = uint64(3)
	mockStatfsFn = getMockStatfsFn(blocks, bSize, bAvail, bFree)

	du, err = DiskUsage(mockStatfsFn)
	test.That(t, err, test.ShouldBeNil)

	expectedUsed = uint64(0)
	expectedFree = uint64(12)
	expectedAvail = uint64(12)
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
	blocks := uint64(3)
	bSize := uint32(4)
	bAvail := uint64(3)
	bFree := uint64(3)
	svc.statfsFn = getMockStatfsFn(blocks, bSize, bAvail, bFree)

	// Replace the default storage check with a more frequent one for testing purposes.
	svc.storageCheckCancelFn()
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	svc.storageCheckCancelFn = cancelFn
	go svc.runStorageCheckAndUpdateCollectors(cancelCtx, time.Millisecond)

	// Load config with an arm, maximum storage percent of 40%, and disabled auto-delete.
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/fake_robot_with_data_manager.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	svc.Update(svc.cancelCtx, conf)
	sleepTime := time.Millisecond * 5
	time.Sleep(sleepTime)

	// Check that collectors are running.
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeTrue)
	test.That(t, svc.maxStoragePercent, test.ShouldEqual, 40)
	test.That(t, svc.enableAutoDelete, test.ShouldBeFalse)

	// Mock 66% disk usage in the system.
	bAvail = uint64(1)
	bFree = uint64(1)
	svc.statfsFn = getMockStatfsFn(blocks, bSize, bAvail, bFree)

	// The running storage check should close any active collectors since we have exceeded
	// the disk usage limit.
	time.Sleep(sleepTime)
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeFalse)

	// Mock 33% disk usage in the system.
	bAvail = uint64(2)
	bFree = uint64(2)
	svc.statfsFn = getMockStatfsFn(blocks, bSize, bAvail, bFree)

	// The running storage check should reactivate collectors since we are now back below
	// the disk usage limit.
	time.Sleep(sleepTime)
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeTrue)

	svc.Close(context.Background())
}

package datamanager

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func (svc *Service) hasActiveCollectors() bool {
	for _, collector := range svc.collectors {
		if collector.Collector != nil {
			return true
		}
	}
	return false
}

type dummyCollector struct {
	closeCount int
}

func (c *dummyCollector) SetTarget(*os.File) {
}

func (c *dummyCollector) GetTarget() *os.File {
	return nil
}

func (c *dummyCollector) Collect() error {
	return nil
}

func (c *dummyCollector) Close() {
	c.closeCount++
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
	_, err := New(context.Background(), r, cfgService, logger)

	test.That(t, err, test.ShouldBeNil)
}

type MockSysCallImplementation struct {
	Blocks uint64
	Bsize  uint32
	Bavail uint64
	Bfree  uint64
}

func (mock MockSysCallImplementation) Statfs(path string, stat *syscall.Statfs_t) error {
	stat.Blocks = mock.Blocks
	stat.Bsize = mock.Bsize
	stat.Bavail = mock.Bavail
	stat.Bfree = mock.Bfree
	return nil
}

func TestDiskUsage(t *testing.T) {
	// Test 50% disk usage.
	blocks := uint64(3)
	bSize := uint32(4)
	bAvail := uint64(1)
	bFree := uint64(2)
	mock := MockSysCallImplementation{blocks, bSize, bAvail, bFree}

	du, err := DiskUsage(mock)
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
	mock = MockSysCallImplementation{blocks, bSize, bAvail, bFree}

	du, err = DiskUsage(mock)
	test.That(t, err, test.ShouldBeNil)

	expectedUsed = uint64(4)
	expectedFree = uint64(8)
	expectedAvail = uint64(8)
	expectedPercentUsed = 33
	test.That(t, du, test.ShouldResemble,
		DiskStatus{expectedAll, expectedUsed, expectedFree, expectedAvail, expectedPercentUsed})
}

func TestRunStorageCheckWithDisabledAutoDeletion(t *testing.T) {
	// Set up a robot with an arm.
	cfg := &Config{
		CaptureDir: "/path/to/capture",
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

	// Add a dummy collector constructor to the service.
	c := &dummyCollector{}
	var dummyCollectorConstructor data.CollectorConstructor = func(
		resource interface{}, params data.CollectorParams) (data.Collector, error) {
		return c, nil
	}
	constructorLookup := func(md data.MethodMetadata) *data.CollectorConstructor {
		return &dummyCollectorConstructor
	}
	dataManager, err := New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	svc := dataManager.(*Service)
	svc.collectorLookup = constructorLookup

	// Load config with an arm, maximum storage percent of 40%, and disabled auto-delete.
	conf, err := config.Read(context.Background(), utils.ResolveFile("robots/configs/fake_robot_with_data_manager.json"), logger)
	test.That(t, err, test.ShouldBeNil)
	svc.storageCheckTicker = time.NewTicker(time.Millisecond)
	svc.Update(svc.cancelCtx, conf)
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeTrue)
	test.That(t, svc.maxStoragePercent, test.ShouldEqual, 40)
	test.That(t, svc.enableAutoDelete, test.ShouldBeFalse)

	// Mock 50% disk usage in the system.
	blocks := uint64(3)
	bSize := uint32(4)
	bAvail := uint64(1)
	bFree := uint64(2)
	svc.sysCall = MockSysCallImplementation{blocks, bSize, bAvail, bFree}

	// A running storage check should close any active collectors since we have exceeded
	// the disk usage limit.
	test.That(t, c.closeCount, test.ShouldEqual, 0)
	go svc.runStorageCheckAndUpdateCollectors()
	time.Sleep(time.Millisecond * 2)
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeFalse)
	test.That(t, c.closeCount, test.ShouldEqual, 1)

	// Mock 33% disk usage in the system.
	bAvail = uint64(2)
	bFree = uint64(2)
	svc.sysCall = MockSysCallImplementation{blocks, bSize, bAvail, bFree}

	// A running storage check should reactivate collectors since we are now back below
	// the disk usage limit.
	time.Sleep(time.Millisecond * 2)
	test.That(t, svc.hasActiveCollectors(), test.ShouldBeTrue)

	svc.storageCheckTicker.Stop()
	svc.cancelFunc()
}

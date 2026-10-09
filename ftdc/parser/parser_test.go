package parser

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/ftdc"
)

func TestHostCPURatio(t *testing.T) {
	readHostCPU := func(timeSecs int64, userSecs, systemSecs, idleSecs, totalSecs float32) map[string]*ratioReading {
		ratios := make(map[string]*ratioReading)
		for _, reading := range []ftdc.Reading{
			{MetricName: "host.CPU.UserSecs", Value: userSecs},
			{MetricName: "host.CPU.SystemSecs", Value: systemSecs},
			{MetricName: "host.CPU.IdleSecs", Value: idleSecs},
			{MetricName: "host.CPU.TotalSecs", Value: totalSecs},
		} {
			test.That(t, pullRatios(reading, timeSecs, ratioMetricToFields, ratios), test.ShouldBeTrue)
		}
		return ratios
	}

	// Simulate two consecutive /proc/stat samples (values in CPU-seconds).
	before := readHostCPU(100, 10, 5, 80, 97)
	after := readHostCPU(105, 11, 5.5, 88, 104)

	// HostUserCPU: (11-10)/(104-97) = 1/7 ≈ 14.29%
	userDiff := after["host.HostUserCPU"].diff(before["host.HostUserCPU"])
	pct, err := userDiff.toValue()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pct, test.ShouldAlmostEqual, float32(1.0/7.0*100), 0.01)

	// HostSystemCPU: (5.5-5)/(104-97) = 0.5/7 ≈ 7.14%
	sysDiff := after["host.HostSystemCPU"].diff(before["host.HostSystemCPU"])
	pct, err = sysDiff.toValue()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pct, test.ShouldAlmostEqual, float32(0.5/7.0*100), 0.01)
}

func TestGCCPURatio(t *testing.T) {
	readGCCPU := func(timeSecs int64, gc, total float32) *ratioReading {
		ratios := make(map[string]*ratioReading)
		for _, reading := range []ftdc.Reading{
			{MetricName: "proc.viam-server.runtime.CPUSecs.GC", Value: gc},
			{MetricName: "proc.viam-server.runtime.CPUSecs.Total", Value: total},
		} {
			test.That(t, pullRatios(reading, timeSecs, ratioMetricToFields, ratios), test.ShouldBeTrue)
		}
		gcCPU, ok := ratios["proc.viam-server.runtime.GCCPU"]
		test.That(t, ok, test.ShouldBeTrue)
		return gcCPU
	}
	before := readGCCPU(100, 1, 40)

	// The runtime refreshes CPU classes only when a GC cycle ends, so most windows see no change.
	_, err := readGCCPU(105, 1, 40).diff(before).toValue()
	test.That(t, err, test.ShouldNotBeNil)

	pct, err := readGCCPU(110, 4, 100).diff(before).toValue()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pct, test.ShouldAlmostEqual, 5)
}

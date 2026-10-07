package parser

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/ftdc"
)

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

package sys

import (
	"runtime"
	"testing"

	"go.viam.com/test"
)

func TestGoRuntimeStatser(t *testing.T) {
	// The CPU class estimates only advance when a GC cycle ends.
	runtime.GC()

	stats, ok := NewGoRuntimeStatser().Stats().(goRuntimeStats)
	test.That(t, ok, test.ShouldBeTrue)

	cpu := stats.CPUSecs
	test.That(t, cpu.Total, test.ShouldBeGreaterThan, 0)
	test.That(t, cpu.GC+cpu.Scavenge+cpu.User+cpu.Idle, test.ShouldAlmostEqual, cpu.Total, 1e-6)
	test.That(t, stats.GCCycles, test.ShouldBeGreaterThan, 0)
	test.That(t, stats.TotalAllocBytes, test.ShouldBeGreaterThan, 0)
	test.That(t, stats.Goroutines, test.ShouldBeGreaterThan, 0)
	test.That(t, stats.GOMAXPROCS, test.ShouldEqual, float64(runtime.GOMAXPROCS(0)))
}

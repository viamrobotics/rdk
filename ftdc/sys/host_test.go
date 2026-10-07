package sys

import (
	"os"
	"runtime"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestHostUsageStatser(t *testing.T) {
	statser, err := NewHostUsageStatser()
	if runtime.GOOS != "linux" {
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, statser == nil, test.ShouldBeTrue)
		return
	}
	test.That(t, err, test.ShouldBeNil)

	// The sleep spans several 10ms /proc/stat ticks so each call has elapsed time to split.
	for range 2 {
		time.Sleep(50 * time.Millisecond)
		hostStats, ok := statser.Stats().(hostStats)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, hostStats.OnlineCPUs, test.ShouldBeGreaterThan, 0)

		cpu := hostStats.CPU
		sum := cpu.UserPct + cpu.SystemPct + cpu.IowaitPct + cpu.IRQPct + cpu.StealPct + cpu.IdlePct
		test.That(t, sum, test.ShouldAlmostEqual, 100, 0.01)
		test.That(t, cpu.MaxCoreBusyPct, test.ShouldBeBetweenOrEqual, 0, 100)

		_, statErr := os.Stat("/proc/pressure/cpu")
		test.That(t, hostStats.Pressure != nil, test.ShouldEqual, statErr == nil)
	}
}

package sys

import (
	"os"
	"runtime"
	"testing"

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

	hostStats, ok := statser.Stats().(hostStats)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, hostStats.OnlineCPUs, test.ShouldBeGreaterThan, 0)

	cpu := hostStats.CPU
	test.That(t, cpu.TotalSecs, test.ShouldBeGreaterThan, 0)
	// TotalSecs must equal the sum of all component CPU-second counts.
	sum := cpu.UserSecs + cpu.SystemSecs + cpu.IowaitSecs + cpu.IRQSecs + cpu.StealSecs + cpu.IdleSecs
	test.That(t, cpu.TotalSecs, test.ShouldAlmostEqual, sum, 1e-6)

	_, statErr := os.Stat("/proc/pressure/cpu")
	test.That(t, hostStats.Pressure != nil, test.ShouldEqual, statErr == nil)
}

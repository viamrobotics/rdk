//go:build linux

package sys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/procfs"
	"go.viam.com/test"
)

func TestHostStatserFromProcFiles(t *testing.T) {
	dir := t.TempDir()
	writeProcFile := func(name, contents string) {
		test.That(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755), test.ShouldBeNil)
		test.That(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600), test.ShouldBeNil)
	}
	writeProcFile("loadavg", "1.50 1.00 0.50 2/300 1234\n")
	writeProcFile("pressure/cpu", "some avg10=12.50 avg60=0.00 avg300=0.00 total=0\n")
	writeProcFile("pressure/io",
		"some avg10=4.00 avg60=0.00 avg300=0.00 total=0\nfull avg10=2.00 avg60=0.00 avg300=0.00 total=0\n")
	// columns: cpu user nice system idle iowait irq softirq steal guest guestnice
	writeProcFile("stat", "cpu  100 0 50 800 10 5 5 0 0 0\n"+
		"cpu0 50 0 25 400 5 3 2 0 0 0\ncpu1 50 0 25 400 5 2 3 0 0 0\n")

	fs, err := procfs.NewFS(dir)
	test.That(t, err, test.ShouldBeNil)
	statser, err := newHostStatser(fs)
	test.That(t, err, test.ShouldBeNil)

	stats := statser.Stats().(hostStats)

	// OnlineCPUs counts individual per-core lines (cpu0, cpu1).
	test.That(t, stats.OnlineCPUs, test.ShouldEqual, 2)
	// procfs converts jiffies to seconds (÷100). Aggregate "cpu" line:
	// user+nice=100j→1s, system=50j→0.5s, iowait=10j→0.1s, irq+softirq=10j→0.1s, steal=0, idle=800j→8s
	test.That(t, stats.CPU.UserSecs, test.ShouldAlmostEqual, 1.0)
	test.That(t, stats.CPU.SystemSecs, test.ShouldAlmostEqual, 0.5)
	test.That(t, stats.CPU.IowaitSecs, test.ShouldAlmostEqual, 0.1)
	test.That(t, stats.CPU.IRQSecs, test.ShouldAlmostEqual, 0.1)
	test.That(t, stats.CPU.StealSecs, test.ShouldAlmostEqual, 0.0)
	test.That(t, stats.CPU.IdleSecs, test.ShouldAlmostEqual, 8.0)
	test.That(t, stats.CPU.TotalSecs, test.ShouldAlmostEqual, 9.7)
	test.That(t, stats.Load1, test.ShouldEqual, 1.5)
	test.That(t, stats.Pressure, test.ShouldResemble, &pressureStats{CPUSomeAvg10: 12.5, IOSomeAvg10: 4, IOFullAvg10: 2})

	// A subsequent call returns new cumulative values directly; cpu2 coming online is visible.
	writeProcFile("stat", "cpu  160 0 70 900 30 5 5 0 0 0\n"+
		"cpu0 100 0 45 410 25 3 2 0 0 0\ncpu1 60 0 25 490 5 2 3 0 0 0\ncpu2 90 0 0 10 0 0 0 0 0 0\n")
	stats = statser.Stats().(hostStats)

	test.That(t, stats.OnlineCPUs, test.ShouldEqual, 3)
	// user+nice=160j→1.6s, system=70j→0.7s, iowait=30j→0.3s, irq+softirq=10j→0.1s, steal=0, idle=900j→9s
	test.That(t, stats.CPU.UserSecs, test.ShouldAlmostEqual, 1.6)
	test.That(t, stats.CPU.SystemSecs, test.ShouldAlmostEqual, 0.7)
	test.That(t, stats.CPU.IowaitSecs, test.ShouldAlmostEqual, 0.3)
	test.That(t, stats.CPU.IRQSecs, test.ShouldAlmostEqual, 0.1)
	test.That(t, stats.CPU.StealSecs, test.ShouldAlmostEqual, 0.0)
	test.That(t, stats.CPU.IdleSecs, test.ShouldAlmostEqual, 9.0)
	test.That(t, stats.CPU.TotalSecs, test.ShouldAlmostEqual, 11.7)
}

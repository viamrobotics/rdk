//go:build linux

package sys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/procfs"
	"go.viam.com/test"
)

func TestCPUShares(t *testing.T) {
	prev := procfs.CPUStat{User: 10, Nice: 1, System: 5, Idle: 100, Iowait: 4, IRQ: 1, SoftIRQ: 1}
	cur := procfs.CPUStat{User: 30, Nice: 1, System: 15, Idle: 160, Iowait: 2, IRQ: 6, SoftIRQ: 6}
	shares, ok := cpuShares(prev, cur)
	test.That(t, ok, test.ShouldBeTrue)

	// A counter that goes backwards, as iowait can, contributes nothing.
	test.That(t, shares, test.ShouldResemble, hostCPUStats{
		UserPct:   20,
		SystemPct: 10,
		IRQPct:    10,
		IdlePct:   60,
	})

	_, ok = cpuShares(cur, cur)
	test.That(t, ok, test.ShouldBeFalse)
}

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
	writeProcFile("stat", "cpu  100 0 50 800 10 5 5 0 0 0\n"+
		"cpu0 50 0 25 400 5 3 2 0 0 0\ncpu1 50 0 25 400 5 2 3 0 0 0\n")

	fs, err := procfs.NewFS(dir)
	test.That(t, err, test.ShouldBeNil)
	statser, err := newHostStatser(fs)
	test.That(t, err, test.ShouldBeNil)

	// cpu0 spends 70 of its 100 ticks busy and cpu1 spends 10 of 100. cpu2 just came online, so
	// its counters (90% busy since boot) have no previous sample to diff against.
	writeProcFile("stat", "cpu  160 0 70 900 30 5 5 0 0 0\n"+
		"cpu0 100 0 45 410 25 3 2 0 0 0\ncpu1 60 0 25 490 5 2 3 0 0 0\ncpu2 90 0 0 10 0 0 0 0 0 0\n")
	stats := statser.Stats().(hostStats)

	test.That(t, stats.OnlineCPUs, test.ShouldEqual, 3)
	test.That(t, stats.CPU.UserPct, test.ShouldAlmostEqual, 30)
	test.That(t, stats.CPU.SystemPct, test.ShouldAlmostEqual, 10)
	test.That(t, stats.CPU.IowaitPct, test.ShouldAlmostEqual, 10)
	test.That(t, stats.CPU.IdlePct, test.ShouldAlmostEqual, 50)
	test.That(t, stats.CPU.MaxCoreBusyPct, test.ShouldAlmostEqual, 70)
	test.That(t, stats.Load1, test.ShouldEqual, 1.5)
	test.That(t, stats.Pressure, test.ShouldResemble, &pressureStats{CPUSomeAvg10: 12.5, IOSomeAvg10: 4, IOFullAvg10: 2})
}

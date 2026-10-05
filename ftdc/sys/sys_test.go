// This test file is  not constrained by a build tag. Each platform has its own
// `newSysUsageStatser` implementation (procfs on linux, gopsutil on darwin and windows), and
// these tests assert the properties all of them must share.

package sys

import (
	"runtime"
	"testing"

	"go.viam.com/test"
)

func TestSelfSysUsageStatser(t *testing.T) {
	// The 32-bit armhf CI job runs under qemu-user, which reports the /proc/<pid>/stat memory
	// fields as 0, so RssMB/VssMB come back 0. That's a qemu limitation, not a 32-bit bug; skip
	// on arm (the test runs normally on amd64/arm64).
	if runtime.GOARCH == "arm" {
		t.Skip("qemu-user hard-codes /proc/<pid>/stat memory fields to 0; can't verify under emulation")
	}

	statser, err := NewSelfSysUsageStatser()
	test.That(t, err, test.ShouldBeNil)

	// The `Statser` contract requires an identical schema on every call, so `Stats` must always
	// return the shared `stats` type rather than a platform-specific struct.
	selfStats, ok := statser.Stats().(stats)
	test.That(t, ok, test.ShouldBeTrue)

	// This process is running, so it necessarily occupies memory. Zero here means the fields were
	// never populated, or were populated in the wrong units.
	test.That(t, selfStats.RssMB, test.ShouldBeGreaterThan, 0)
	test.That(t, selfStats.VssMB, test.ShouldBeGreaterThan, 0)

	// ElapsedTimeSecs must be this process' *age*
	test.That(t, selfStats.ElapsedTimeSecs, test.ShouldBeGreaterThan, 0)
	test.That(t, selfStats.ElapsedTimeSecs, test.ShouldBeLessThan, 3600)

	// CPU times can round to zero on a short-lived process, so only assert they are
	// not negative.
	test.That(t, selfStats.UserCPUSecs, test.ShouldBeGreaterThanOrEqualTo, 0)
	test.That(t, selfStats.SystemCPUSecs, test.ShouldBeGreaterThanOrEqualTo, 0)
}

func TestSysUsageStatserInvalidPid(t *testing.T) {
	// A negative pid cannot exist on any platform. The constructor must reject it
	_, err := NewSysUsageStatser(-1)
	test.That(t, err, test.ShouldNotBeNil)
}

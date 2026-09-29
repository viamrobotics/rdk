// This test file is not constrained by a build tag. Unlike the process statser, the
// network statser's contract  differs by platform, darwin declines to collect,
// so the test branches on `runtime.GOOS` rather than asserting one shared set of properties.

package sys

import (
	"runtime"
	"testing"

	"go.viam.com/test"
)

func TestNetUsageStatser(t *testing.T) {
	statser, err := NewNetUsageStatser()

	if runtime.GOOS == "darwin" {
		// darwin deliberately collects nothing: gopsutil's darwin implementation shells out to
		// `netstat` and `lsof`, which is expensive to run once a second. Declining with an
		// error keeps the `net` section out of FTDC entirely. A statser reporting zeroes would
		// indistinguishable from a genuinely idle network.
		test.That(t, err, test.ShouldNotBeNil)

		// Compared against nil directly rather than via `ShouldBeNil`, which also accepts a nil
		// pointer boxed in a non-nil interface.
		test.That(t, statser == nil, test.ShouldBeTrue)
		return
	}

	test.That(t, err, test.ShouldBeNil)

	// The `Statser` contract requires an identical schema on every call, so `Stats` must always
	// return the shared `networkStats` type rather than a platform-specific struct.
	netStats, ok := statser.Stats().(networkStats)
	test.That(t, ok, test.ShouldBeTrue)

	// Every machine has a loopback interface, so an empty map means the platform's
	// interface counters were never populated rather than that the machine is quiet.
	test.That(t, len(netStats.Ifaces), test.ShouldBeGreaterThan, 0)
}

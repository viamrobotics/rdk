package diskusage

import (
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestEnoughFreeSpace(t *testing.T) {
	// The temp dir's real volume has far more than a few bytes free, but less than a huge threshold.
	dir := t.TempDir()

	t.Run("plenty of free space", func(t *testing.T) {
		enough, available, err := EnoughFreeSpace(dir, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, enough, test.ShouldBeTrue)
		test.That(t, available, test.ShouldBeGreaterThan, uint64(0))
	})

	t.Run("threshold larger than disk", func(t *testing.T) {
		enough, available, err := EnoughFreeSpace(dir, 1<<62)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, enough, test.ShouldBeFalse)
		test.That(t, available, test.ShouldBeGreaterThan, uint64(0))
	})

	t.Run("path that does not exist yet", func(t *testing.T) {
		// A not-yet-created dir must report its nearest existing ancestor's volume rather than
		// erroring, which would silently disable the guard.
		missing := filepath.Join(dir, "does", "not", "exist", "yet")
		enough, available, err := EnoughFreeSpace(missing, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, enough, test.ShouldBeTrue)
		test.That(t, available, test.ShouldBeGreaterThan, uint64(0))
	})
}

func TestIsLowOnSpace(t *testing.T) {
	// We can't control the host's disk fill, so only assert the call succeeds and reports a real
	// volume (threshold logic is covered by TestLowOnSpaceThresholds), and that a not-yet-created
	// path resolves to its nearest existing ancestor rather than erroring.
	dir := t.TempDir()
	usage, _, err := IsLowOnSpace(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, usage.SizeBytes, test.ShouldBeGreaterThan, uint64(0))

	missingUsage, _, err := IsLowOnSpace(filepath.Join(dir, "does", "not", "exist"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, missingUsage.SizeBytes, test.ShouldEqual, usage.SizeBytes)
}

func TestLowOnSpaceThresholds(t *testing.T) {
	// Exercise the real decision (DiskUsage.IsLow, which IsLowOnSpace delegates to) against
	// synthetic values so we don't depend on the host's actual disk state.

	// Plenty of bytes and well under 90% used: healthy.
	test.That(t, DiskUsage{AvailableBytes: 50 * gb, SizeBytes: 100 * gb}.IsLow(), test.ShouldBeFalse)
	// Huge disk but exactly 90% used (10% free): low on the utilization rule even
	// though many GB remain free.
	test.That(t, DiskUsage{AvailableBytes: 10 * gb, SizeBytes: 100 * gb}.IsLow(), test.ShouldBeTrue)
	// Small disk, only a few MB free: low on the absolute-bytes rule even though
	// utilization is moderate.
	test.That(t, DiskUsage{AvailableBytes: 5 * mb, SizeBytes: 20 * mb}.IsLow(), test.ShouldBeTrue)
	// Zero total size = pseudo-fs/garbage statfs result, not a real volume: not low, so we don't
	// warn (or block) every interval despite AvailableBytes being under the floor.
	test.That(t, DiskUsage{AvailableBytes: 0, SizeBytes: 0}.IsLow(), test.ShouldBeFalse)
}

func TestDiskUsage(t *testing.T) {
	t.Run("String()", func(t *testing.T) {
		type testCase struct {
			du  DiskUsage
			exp string
		}

		tcs := []testCase{
			{
				du:  DiskUsage{AvailableBytes: 100, SizeBytes: 100},
				exp: "diskusage.DiskUsage{Available: 100 Bytes, Size: 100 Bytes, AvailablePercent: 100.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 100, SizeBytes: 10000},
				exp: "diskusage.DiskUsage{Available: 100 Bytes, Size: 10.00 KB, AvailablePercent: 1.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 1000000, SizeBytes: 10000000},
				exp: "diskusage.DiskUsage{Available: 1.00 MB, Size: 10.00 MB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000, SizeBytes: 100000000000},
				exp: "diskusage.DiskUsage{Available: 10.00 GB, Size: 100.00 GB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000000, SizeBytes: 100000000000000},
				exp: "diskusage.DiskUsage{Available: 10.00 TB, Size: 100.00 TB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000000000, SizeBytes: 100000000000000000},
				exp: "diskusage.DiskUsage{Available: 10000.00 TB, Size: 100000.00 TB, AvailablePercent: 10.00%}",
			},
		}

		for _, tc := range tcs {
			test.That(t, tc.du.String(), test.ShouldResemble, tc.exp)
		}
	})
}

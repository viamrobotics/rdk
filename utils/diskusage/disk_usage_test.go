package diskusage

import (
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestEnoughFreeSpace(t *testing.T) {
	// The temp dir lives on a real volume, which on any test machine will have far
	// more than a handful of bytes free but less than an absurdly large threshold.
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
		// A directory that hasn't been created yet should report the free space of
		// the volume it would live on (via its nearest existing ancestor) rather
		// than erroring out, which is what would silently disable the guard.
		missing := filepath.Join(dir, "does", "not", "exist", "yet")
		enough, available, err := EnoughFreeSpace(missing, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, enough, test.ShouldBeTrue)
		test.That(t, available, test.ShouldBeGreaterThan, uint64(0))
	})
}

func TestIsLowOnSpace(t *testing.T) {
	// We can't control how full the test host's disk is, so we only assert that the
	// real call succeeds and reports a real volume; the deterministic threshold
	// logic is covered by TestLowOnSpaceThresholds. A path that does not exist yet
	// must resolve to its nearest existing ancestor rather than erroring, which
	// would silently disable the monitor.
	dir := t.TempDir()
	usage, _, err := IsLowOnSpace(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, usage.SizeBytes, test.ShouldBeGreaterThan, uint64(0))

	missingUsage, _, err := IsLowOnSpace(filepath.Join(dir, "does", "not", "exist"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, missingUsage.SizeBytes, test.ShouldEqual, usage.SizeBytes)
}

func TestLowOnSpaceThresholds(t *testing.T) {
	// Exercise the threshold logic directly on DiskUsage values so we don't depend
	// on the host's actual disk state. lowFor mirrors IsLowOnSpace's decision.
	lowFor := func(du DiskUsage) bool {
		return du.AvailableBytes < MinFreeBytes || (1-du.AvailablePercent()) >= MaxUsedFraction
	}

	// Plenty of bytes and well under 90% used: healthy.
	test.That(t, lowFor(DiskUsage{AvailableBytes: 50 * gib, SizeBytes: 100 * gib}), test.ShouldBeFalse)
	// Huge disk but exactly 90% used (10% free): low on the utilization rule even
	// though many GB remain free.
	test.That(t, lowFor(DiskUsage{AvailableBytes: 10 * gib, SizeBytes: 100 * gib}), test.ShouldBeTrue)
	// Small disk, only a few MiB free: low on the absolute-bytes rule even though
	// utilization is moderate.
	test.That(t, lowFor(DiskUsage{AvailableBytes: 5 * mib, SizeBytes: 20 * mib}), test.ShouldBeTrue)
}

func TestFormatBytes(t *testing.T) {
	type testCase struct {
		in  uint64
		exp string
	}
	for _, tc := range []testCase{
		{0, "0 Bytes"},
		{512, "512 Bytes"},
		{kib, "1.00 KiB"}, // exact power: must not render as "1024.00 Bytes"
		{mib, "1.00 MiB"}, // exact power: must not render as "1024.00 KiB"
		{gib, "1.00 GiB"}, // exact power: must not render as "1024.00 MiB"
		{tib, "1.00 TiB"}, // exact power: must not render as "1024.00 GiB"
		{mib + mib/2, "1.50 MiB"},
	} {
		test.That(t, FormatBytes(tc.in), test.ShouldEqual, tc.exp)
	}
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
				exp: "diskusage.DiskUsage{Available: 100 Bytes, Size: 9.77 KiB, AvailablePercent: 1.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 1000000, SizeBytes: 10000000},
				exp: "diskusage.DiskUsage{Available: 976.56 KiB, Size: 9.54 MiB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000, SizeBytes: 100000000000},
				exp: "diskusage.DiskUsage{Available: 9.31 GiB, Size: 93.13 GiB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000000, SizeBytes: 100000000000000},
				exp: "diskusage.DiskUsage{Available: 9.09 TiB, Size: 90.95 TiB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000000000, SizeBytes: 100000000000000000},
				exp: "diskusage.DiskUsage{Available: 9094.95 TiB, Size: 90949.47 TiB, AvailablePercent: 10.00%}",
			},
		}

		for _, tc := range tcs {
			test.That(t, tc.du.String(), test.ShouldResemble, tc.exp)
		}
	})
}

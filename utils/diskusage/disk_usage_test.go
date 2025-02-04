package diskusage

import (
	"testing"

	"go.viam.com/test"
)

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
				exp: "diskusage.DiskUsage{Available: 100 Bytes, Size: 9.765625 KB, AvailablePercent: 1.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 1000000, SizeBytes: 10000000},
				exp: "diskusage.DiskUsage{Available: 976.562500 KB, Size: 9.536743 MB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000, SizeBytes: 100000000000},
				exp: "diskusage.DiskUsage{Available: 9.313226 GB, Size: 93.132257 GB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000000, SizeBytes: 100000000000000},
				exp: "diskusage.DiskUsage{Available: 9.094947 TB, Size: 90.949470 TB, AvailablePercent: 10.00%}",
			},
			{
				du:  DiskUsage{AvailableBytes: 10000000000000000, SizeBytes: 100000000000000000},
				exp: "diskusage.DiskUsage{Available: 9094.947018 TB, Size: 90949.470177 TB, AvailablePercent: 10.00%}",
			},
		}

		for _, tc := range tcs {
			test.That(t, tc.du.String(), test.ShouldResemble, tc.exp)
		}
	})
}

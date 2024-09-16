package sync

import (
	"testing"
	"time"

	"go.viam.com/test"
)

const (
	_ = 1 << (10 * iota)
	kib
	mib
	gib
	tib
)

func TestNewUploadStats(t *testing.T) {
	empty := &atomicUploadStats{}
	test.That(t, newUploadStats(empty), test.ShouldResemble, uploadStats{})
	test.That(t, newUploadStats(populatedAtomicUploadStats()), test.ShouldResemble, uploadStats{
		arbitrary: stat{uploadedFileCount: 1, uploadedBytes: 2, uploadFailedFileCount: 3},
		binary:    stat{uploadedFileCount: 4, uploadedBytes: 5, uploadFailedFileCount: 6},
		tabular:   stat{uploadedFileCount: 7, uploadedBytes: 8, uploadFailedFileCount: 9},
	})
}

func TestSummary(t *testing.T) {
	type testCase struct {
		name     string
		prev     uploadStats
		curr     uploadStats
		interval time.Duration
		exp      []string
	}
	empty := uploadStats{}
	tcs := []testCase{
		{
			name:     "empty to empty",
			prev:     empty,
			curr:     empty,
			interval: time.Minute,
			exp: []string{
				"total uploads: 0, rate: 0.00/sec",
				"total uploaded: 0 Bytes, rate: 0 Bytes/sec",
				"total failed uploads: 0, rate: 0.00/sec",
			},
		},
		{
			name:     "empty to fully populated",
			prev:     empty,
			curr:     populatedUploadStats(),
			interval: time.Minute,
			exp: []string{
				"total uploads: 12, rate: 0.20/sec",
				"total uploaded: 1.00 GB, rate: 17.08 MB/sec",
				"total failed uploads: 18, rate: 0.30/sec",
				"arbitrary file, (files uploaded): total: 1, rate: 0.016667/sec",
				"arbitrary file, (uploaded): total: 1.00 GB, rate: 17.07 MB/sec",
				"arbitrary file, (failed file uploads): total: 3, rate: 0.050000/sec",
				"binary file, (files uploaded): total: 4, rate: 0.066667/sec",
				"binary file, (uploaded): total: 1.00 MB, rate: 17.07 KB/sec",
				"binary file, (failed file uploads): total: 6, rate: 0.100000/sec",
				"tabular file, (files uploaded): total: 7, rate: 0.116667/sec",
				"tabular file, (uploaded): total: 1.00 KB, rate: 17 Bytes/sec",
				"tabular file, (failed file uploads): total: 9, rate: 0.150000/sec",
			},
		},
		{
			name: "empty to paritially populated",
			prev: empty,
			curr: uploadStats{
				arbitrary: stat{uploadedFileCount: 1, uploadedBytes: gib + 1},
			},
			interval: time.Minute,
			exp: []string{
				"total uploads: 1, rate: 0.02/sec",
				"total uploaded: 1.00 GB, rate: 17.07 MB/sec",
				"total failed uploads: 0, rate: 0.00/sec",
				"arbitrary file, (files uploaded): total: 1, rate: 0.016667/sec",
				"arbitrary file, (uploaded): total: 1.00 GB, rate: 17.07 MB/sec",
				"arbitrary file, (failed file uploads): total: 0, rate: 0.000000/sec",
			},
		},
		{
			name:     "fully populated to fully populated",
			prev:     populatedUploadStats(),
			curr:     populatedUploadStats(),
			interval: time.Minute,
			exp: []string{
				"total uploads: 12, rate: 0.00/sec",
				"total uploaded: 1.00 GB, rate: 0 Bytes/sec",
				"total failed uploads: 18, rate: 0.00/sec",
				"arbitrary file, (files uploaded): total: 1, rate: 0.000000/sec",
				"arbitrary file, (uploaded): total: 1.00 GB, rate: 0 Bytes/sec",
				"arbitrary file, (failed file uploads): total: 3, rate: 0.000000/sec",
				"binary file, (files uploaded): total: 4, rate: 0.000000/sec",
				"binary file, (uploaded): total: 1.00 MB, rate: 0 Bytes/sec",
				"binary file, (failed file uploads): total: 6, rate: 0.000000/sec",
				"tabular file, (files uploaded): total: 7, rate: 0.000000/sec",
				"tabular file, (uploaded): total: 1.00 KB, rate: 0 Bytes/sec",
				"tabular file, (failed file uploads): total: 9, rate: 0.000000/sec",
			},
		},
		{
			name: "paritially populated to fully populated",
			prev: uploadStats{
				binary: stat{uploadedFileCount: 1, uploadedBytes: kib + 1},
			},
			curr:     populatedUploadStats(),
			interval: time.Minute,
			exp: []string{
				"total uploads: 12, rate: 0.18/sec",
				"total uploaded: 1.00 GB, rate: 17.08 MB/sec",
				"total failed uploads: 18, rate: 0.30/sec",
				"arbitrary file, (files uploaded): total: 1, rate: 0.016667/sec",
				"arbitrary file, (uploaded): total: 1.00 GB, rate: 17.07 MB/sec",
				"arbitrary file, (failed file uploads): total: 3, rate: 0.050000/sec",
				"binary file, (files uploaded): total: 4, rate: 0.050000/sec",
				"binary file, (uploaded): total: 1.00 MB, rate: 17.05 KB/sec",
				"binary file, (failed file uploads): total: 6, rate: 0.100000/sec",
				"tabular file, (files uploaded): total: 7, rate: 0.116667/sec",
				"tabular file, (uploaded): total: 1.00 KB, rate: 17 Bytes/sec",
				"tabular file, (failed file uploads): total: 9, rate: 0.150000/sec",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res := summary(tc.prev, tc.curr, tc.interval)
			test.That(t, res, test.ShouldResemble, tc.exp)
		})
	}
}

func populatedAtomicUploadStats() *atomicUploadStats {
	atomicStats := &atomicUploadStats{}
	atomicStats.arbitrary.uploadedFileCount.Add(1)
	atomicStats.arbitrary.uploadedBytes.Add(2)
	atomicStats.arbitrary.uploadFailedFileCount.Add(3)
	atomicStats.binary.uploadedFileCount.Add(4)
	atomicStats.binary.uploadedBytes.Add(5)
	atomicStats.binary.uploadFailedFileCount.Add(6)
	atomicStats.tabular.uploadedFileCount.Add(7)
	atomicStats.tabular.uploadedBytes.Add(8)
	atomicStats.tabular.uploadFailedFileCount.Add(9)
	return atomicStats
}

func populatedUploadStats() uploadStats {
	return uploadStats{
		arbitrary: stat{uploadedFileCount: 1, uploadedBytes: gib + 1, uploadFailedFileCount: 3},
		binary:    stat{uploadedFileCount: 4, uploadedBytes: mib + 1, uploadFailedFileCount: 6},
		tabular:   stat{uploadedFileCount: 7, uploadedBytes: kib + 1, uploadFailedFileCount: 9},
	}
}

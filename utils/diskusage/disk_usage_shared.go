package diskusage

import (
	"fmt"
)

// MinFreeBytes is the minimum amount of free space, in bytes, that viam-server
// expects to have available before it will download anything to disk. Below this
// threshold downloads are skipped and a warning is logged.
const MinFreeBytes uint64 = 10 * mib

// EnoughFreeSpace reports whether the filesystem/volume that path lives on has at
// least minBytes available to an unprivileged user. The returned available value
// is the number of bytes currently available on that volume, which callers can use
// for logging. Note that Statfs reports usage for the whole volume containing path,
// not just the directory at path.
func EnoughFreeSpace(path string, minBytes uint64) (enough bool, available uint64, err error) {
	usage, err := Statfs(path)
	if err != nil {
		return false, 0, err
	}
	return usage.AvailableBytes >= minBytes, usage.AvailableBytes, nil
}

// FormatBytes renders a byte count in human-friendly units (KB/MB/GB/TB).
func FormatBytes(b uint64) string {
	return formatBytesU64(b)
}

// DiskUsage contains usage data and provides user-friendly access methods.
type DiskUsage struct {
	// AvailableBytes is the total available bytes on file system to an unprivileged user.
	AvailableBytes uint64
	// SizeBytes is the total size of the file system in bytes.
	SizeBytes uint64
}

func (du DiskUsage) String() string {
	return fmt.Sprintf("diskusage.DiskUsage{Available: %s, Size: %s, AvailablePercent: %.2f",
		formatBytesU64(du.AvailableBytes), formatBytesU64(du.SizeBytes), du.AvailablePercent()*100) + "%}"
}

// AvailablePercent returns the percentage (0.0-1.0) of the disk available
// to an unprivileged user
// see `man statfs` for how the underlying values are derived on your platform.
func (du DiskUsage) AvailablePercent() float64 {
	return float64(du.AvailableBytes) / float64(du.SizeBytes)
}

const (
	_ = 1 << (10 * iota)
	kib
	mib
	gib
	tib
)

func formatBytesU64(b uint64) string {
	switch {
	case b > tib:
		return fmt.Sprintf("%f TB", float64(b)/tib)
	case b > gib:
		return fmt.Sprintf("%f GB", float64(b)/gib)
	case b > mib:
		return fmt.Sprintf("%f MB", float64(b)/mib)
	case b > kib:
		return fmt.Sprintf("%f KB", float64(b)/kib)
	default:
		return fmt.Sprintf("%d Bytes", b)
	}
}

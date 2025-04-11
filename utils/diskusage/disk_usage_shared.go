package diskusage

import (
	"fmt"
)

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

package diskusage

import (
	"fmt"
	"os"
	"path/filepath"
)

// MinFreeBytes is the floor of free space viam-server tries to keep on volumes it writes to
// (downloads, local copies, unpacking). Falling below it always logs a warning, and refuses the
// install only when VIAM_ENABLE_DISK_SPACE_BLOCK is set (otherwise log-only). Also a trigger for
// the background monitor; see IsLow.
const MinFreeBytes uint64 = 10 * mb

// MaxUsedFraction is the utilization (0.0-1.0) at or above which the monitor flags a volume as
// low, regardless of absolute bytes free.
const MaxUsedFraction = 0.90

// This package has two non-interchangeable notions of "low on space", both via Usage (which
// resolves a not-yet-created path to its nearest existing ancestor first):
//
//   - IsLow / IsLowOnSpace — health check: at/above MaxUsedFraction utilization OR under
//     MinFreeBytes free.
//   - EnoughFreeSpace — install guard: whether a specific byte count is available (no
//     utilization rule), since callers pass a concrete requirement.
//
// "Available" is statfs f_bavail (unprivileged-usable), so running as root makes these a
// conservative lower bound — the safe direction for a guard.

// Usage resolves path to its nearest existing ancestor (path need not exist yet) and
// returns the usage of the volume it lives on. Statfs reports usage for the whole volume
// containing path, not just the directory at path.
func Usage(path string) (DiskUsage, error) {
	return Statfs(nearestExistingDir(path))
}

// IsLowOnSpace reports whether the volume holding path is low on disk space (see IsLow)
// and returns the underlying usage so callers can log it.
func IsLowOnSpace(path string) (usage DiskUsage, low bool, err error) {
	usage, err = Usage(path)
	if err != nil {
		return DiskUsage{}, false, err
	}
	return usage, usage.IsLow(), nil
}

// IsLow reports whether this usage is low: at/above MaxUsedFraction utilization or under
// MinFreeBytes free. Split out from IsLowOnSpace so it can be tested on synthetic values.
// Reserved blocks count as used, so utilization is slightly overestimated (warns early). A
// zero SizeBytes is treated as not-low (see below) rather than assessed.
func (du DiskUsage) IsLow() bool {
	// A zero total size is a pseudo-fs (procfs/sysfs) or garbage statfs result, not a real volume
	// we can assess — treat it as not-low rather than warning every interval. (Mirrors
	// checkDiskSpace, which proceeds on an outright statfs error.)
	if du.SizeBytes == 0 {
		return false
	}
	usedFraction := 1 - du.AvailablePercent()
	return du.AvailableBytes < MinFreeBytes || usedFraction >= MaxUsedFraction
}

// EnoughFreeSpace reports whether the volume that path lives on has at least minBytes
// available to an unprivileged user, returning that available figure for logging.
func EnoughFreeSpace(path string, minBytes uint64) (enough bool, available uint64, err error) {
	usage, err := Usage(path)
	if err != nil {
		return false, 0, err
	}
	// Don't refuse an install on a pseudo-fs/garbage (zero total size) result; let ENOSPC be the
	// backstop, consistent with how checkDiskSpace handles a statfs error.
	if usage.SizeBytes == 0 {
		return true, usage.AvailableBytes, nil
	}
	return usage.AvailableBytes >= minBytes, usage.AvailableBytes, nil
}

// nearestExistingDir walks up from path until it finds something that exists,
// returning that ancestor. If no ancestor exists (e.g. an empty path), it returns
// path unchanged and lets the subsequent Statfs surface the error.
func nearestExistingDir(path string) string {
	for path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return path
}

// FormatBytes renders a byte count in human-friendly units (KB/MB/GB/TB).
func FormatBytes(b uint64) string {
	return formatBytesU64(b)
}

// FormatBytesI64 renders a signed byte count in human-friendly units
// (KB/MB/GB/TB), preserving the sign for negative values.
func FormatBytesI64(b int64) string {
	if b < 0 {
		return "-" + formatBytesU64(uint64(-b))
	}
	return formatBytesU64(uint64(b))
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

// Decimal (1000-based) units, so the KB/MB/GB/TB labels below are accurate
// (1 KB == 1000 bytes, not 1024).
const (
	kb = 1000
	mb = 1000 * kb
	gb = 1000 * mb
	tb = 1000 * gb
)

func formatBytesU64(b uint64) string {
	// The comparisons use >= so an exact power (e.g. 1 MB) renders in its own unit
	// instead of falling through to "1000.00 KB".
	switch {
	case b >= tb:
		return fmt.Sprintf("%.2f TB", float64(b)/tb)
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.2f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.2f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d Bytes", b)
	}
}

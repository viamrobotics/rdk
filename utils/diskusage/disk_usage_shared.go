package diskusage

import (
	"fmt"
	"os"
	"path/filepath"
)

// MinFreeBytes is the minimum amount of free space, in bytes, that viam-server
// expects to have available before it will download anything to disk. Below this
// threshold downloads are skipped and a warning is logged.
const MinFreeBytes uint64 = 10 * mib

// MaxUsedFraction is the volume-utilization fraction (0.0-1.0) at or above which
// the disk-space monitor considers a volume low on space, independent of how many
// absolute bytes remain free. At/above this utilization a warning is logged.
const MaxUsedFraction = 0.90

// IsLowOnSpace reports whether the volume holding path is low on disk space and
// returns the underlying usage so callers can log it. "Low" means the volume is at
// or above MaxUsedFraction utilization, or has fewer than MinFreeBytes available.
// path need not exist yet; see EnoughFreeSpace for how a not-yet-created path is
// resolved to the volume it would live on.
func IsLowOnSpace(path string) (usage DiskUsage, low bool, err error) {
	usage, err = Statfs(nearestExistingDir(path))
	if err != nil {
		return DiskUsage{}, false, err
	}
	// usedFraction treats reserved blocks as used (we only have available-to-unprivileged
	// and total), so this slightly overestimates utilization and errs toward warning early.
	// If SizeBytes is 0 the fraction is NaN and the comparison is false, leaving the
	// absolute-bytes check as the sole trigger.
	usedFraction := 1 - usage.AvailablePercent()
	low = usage.AvailableBytes < MinFreeBytes || usedFraction >= MaxUsedFraction
	return usage, low, nil
}

// EnoughFreeSpace reports whether the filesystem/volume that path lives on has at
// least minBytes available to an unprivileged user. The returned available value
// is the number of bytes currently available on that volume, which callers can use
// for logging. Note that Statfs reports usage for the whole volume containing path,
// not just the directory at path.
//
// path need not exist yet: Statfs requires an existing path, so we walk up to the
// nearest existing ancestor before measuring. A directory that has not been created
// yet lives on the same volume as its nearest existing parent, so this reports the
// volume it would be created on rather than failing with a not-found error.
//
// The available figure is the space reported as usable by an unprivileged user
// (statfs f_bavail). A process running as root can typically write into the
// filesystem's reserved blocks too, so when viam-server runs as root this is a
// conservative lower bound: the check may report "not enough" while root could in
// fact still write. That is the safe direction to err for a download guard.
func EnoughFreeSpace(path string, minBytes uint64) (enough bool, available uint64, err error) {
	usage, err := Statfs(nearestExistingDir(path))
	if err != nil {
		return false, 0, err
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

// FormatBytes renders a byte count in human-friendly units (KiB/MiB/GiB/TiB).
func FormatBytes(b uint64) string {
	return formatBytesU64(b)
}

// FormatBytesI64 renders a signed byte count in human-friendly units
// (KiB/MiB/GiB/TiB), preserving the sign for negative values.
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

const (
	_ = 1 << (10 * iota)
	kib
	mib
	gib
	tib
)

func formatBytesU64(b uint64) string {
	// Units are binary (1024-based), so the labels are KiB/MiB/etc. rather than
	// KB/MB. The comparisons use >= so that an exact power (e.g. 1 MiB) is rendered
	// in its own unit instead of falling through to "1024.000 KiB".
	switch {
	case b >= tib:
		return fmt.Sprintf("%.2f TiB", float64(b)/tib)
	case b >= gib:
		return fmt.Sprintf("%.2f GiB", float64(b)/gib)
	case b >= mib:
		return fmt.Sprintf("%.2f MiB", float64(b)/mib)
	case b >= kib:
		return fmt.Sprintf("%.2f KiB", float64(b)/kib)
	default:
		return fmt.Sprintf("%d Bytes", b)
	}
}

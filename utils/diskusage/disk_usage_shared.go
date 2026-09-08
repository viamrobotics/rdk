package diskusage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
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
	// CheckDiskSpace, which proceeds on an outright statfs error.)
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
	// backstop, consistent with how CheckDiskSpace handles a statfs error.
	if usage.SizeBytes == 0 {
		return true, usage.AvailableBytes, nil
	}
	return usage.AvailableBytes >= minBytes, usage.AvailableBytes, nil
}

// ErrInsufficientDiskSpace is returned by CheckDiskSpace when blocking is on and the volume is
// low. Callers use errors.Is to tell a disk-space refusal from other failures (e.g. a corrupt
// archive) and surface an accurate message.
var ErrInsufficientDiskSpace = errors.New("not enough free disk space")

// EnoughFreeSpace reports whether the volume holding path has at least minBytes
// available. It is a package var so tests can inject a low-space result without
// having to actually fill a disk.
var EnoughFreeSpaceFunc = EnoughFreeSpace

// blockingEnabled reports whether low-space conditions should refuse the operation (download,
// local copy, or unpack). Default (unset) is false: low-space is logged but the operation
// proceeds (log-only). See utils.ViamEnableDiskSpaceBlockEnvVar.
func blockingEnabled() bool {
	return utils.GetenvBool(utils.ViamEnableDiskSpaceBlockEnvVar, false)
}

// CheckDiskSpace checks whether the volume holding path has required bytes free. It returns
// low=true whenever space is low. When blocking is enabled via ViamEnableDiskSpaceBlockEnvVar it
// returns an error refusing the op (the caller logs it, so CheckDiskSpace stays quiet to avoid
// double-logging the same reason every cycle); otherwise it logs a warning and returns nil so the
// op proceeds (log-only). A failed check is logged and treated as "proceed" so a broken statfs
// never blocks installs. desc names the op in logs/errors; extraFields extend the warning.
func CheckDiskSpace(logger logging.Logger, path, desc string, required uint64, extraFields ...any) (low bool, err error) {
	enough, available, err := EnoughFreeSpaceFunc(path, required)
	if err != nil {
		logger.Warnw("could not check free disk space; proceeding",
			append([]any{"desc", desc, "path", path, "error", err}, extraFields...)...)
		return false, nil
	}
	if enough {
		return false, nil
	}
	if !blockingEnabled() {
		// Log-only: the op proceeds and returns no error, so this warning is the only signal
		// that space is low.
		logger.Warnw("not enough free disk space",
			append([]any{
				"desc", desc, "path", path,
				"available", utils.FormatBytes(available),
				"required", utils.FormatBytes(required),
				"blocking", false,
			}, extraFields...)...)
		return true, nil
	}
	// Blocking: don't warn here — the returned error carries the same detail and is logged by the
	// caller (cloud_package_manager.go and local_package_manager.go both log the install error),
	// so warning too would double-log the same reason every sync cycle.
	return true, fmt.Errorf("%w for %s: %s available, %s required",
		ErrInsufficientDiskSpace, desc, utils.FormatBytes(available), utils.FormatBytes(required))
}

// nearestExistingDir walks up from path until it finds an existing directory,
// returning that ancestor. Non-directories are skipped
func nearestExistingDir(path string) string {
	for path != "" {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
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

// DiskUsage contains usage data and provides user-friendly access methods.
type DiskUsage struct {
	// AvailableBytes is the total available bytes on file system to an unprivileged user.
	AvailableBytes uint64
	// SizeBytes is the total size of the file system in bytes.
	SizeBytes uint64
}

func (du DiskUsage) String() string {
	return fmt.Sprintf("diskusage.DiskUsage{Available: %s, Size: %s, AvailablePercent: %.2f",
		utils.FormatBytes(du.AvailableBytes), utils.FormatBytes(du.SizeBytes), du.AvailablePercent()*100) + "%}"
}

// AvailablePercent returns the percentage (0.0-1.0) of the disk available
// to an unprivileged user
// see `man statfs` for how the underlying values are derived on your platform.
func (du DiskUsage) AvailablePercent() float64 {
	return float64(du.AvailableBytes) / float64(du.SizeBytes)
}

// Decimal (1000-based) units, used for the free-space floor (MinFreeBytes) and thresholds.
// The human-friendly formatting that renders these labels lives in utils.FormatBytes.
const (
	kb = 1000
	mb = 1000 * kb
	gb = 1000 * mb
	tb = 1000 * gb
)

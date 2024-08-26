//go:build !windows

// Package diskusage is used to get platform specific file system usage information.
package diskusage

import "syscall"

// DiskUsage contains usage data and provides user-friendly access methods.
type DiskUsage struct {
	// AvailableBytes is the total available bytes on file system to an unprivileged user.
	AvailableBytes uint64
	// SizeBytes is the total size of the file system in bytes.
	SizeBytes uint64
}

// Statfs returns file system statistics.
func Statfs(volumePath string) (DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(volumePath, &stat); err != nil {
		return DiskUsage{}, err
	}
	return DiskUsage{
		AvailableBytes: stat.Bavail * uint64(stat.Bsize),
		SizeBytes:      stat.Blocks * uint64(stat.Bsize),
	}, nil
}

// AvailablePercent returns the percentage (0.0-1.0) of the disk available
// to an unprivileged user
// see `man statfs` for how the underlying values are derived on your platform.
func (du DiskUsage) AvailablePercent() float64 {
	return float64(du.AvailableBytes) / float64(du.SizeBytes)
}

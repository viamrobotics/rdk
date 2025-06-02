//go:build !windows

// Package diskusage is used to get platform specific file system usage information.
package diskusage

import (
	"syscall"
)

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

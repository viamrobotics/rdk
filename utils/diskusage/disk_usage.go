//go:build !windows

// Package diskusage is used to get platform specific file system usage information.
package diskusage

import "syscall"

// DiskUsage contains usage data and provides user-friendly access methods.
type DiskUsage struct {
	stat *syscall.Statfs_t
}

// NewDiskUsage returns an object holding the disk usage of volumePath
// or nil in case of error (invalid path, etc).
func NewDiskUsage(volumePath string) *DiskUsage {
	var stat syscall.Statfs_t
	//nolint:errcheck,gosec
	syscall.Statfs(volumePath, &stat)
	return &DiskUsage{&stat}
}

// Free returns total free bytes on file system.
func (du *DiskUsage) Free() uint64 {
	return du.stat.Bfree * uint64(du.stat.Bsize)
}

// Available return total available bytes on file system to an unprivileged user.
func (du *DiskUsage) Available() uint64 {
	return du.stat.Bavail * uint64(du.stat.Bsize)
}

// Size returns total size of the file system.
func (du *DiskUsage) Size() uint64 {
	return du.stat.Blocks * uint64(du.stat.Bsize)
}

// Used returns total bytes used in file system.
func (du *DiskUsage) Used() uint64 {
	return du.Size() - du.Free()
}

// Usage returns percentage of use on the file system.
func (du *DiskUsage) Usage() float32 {
	return float32(du.Used()) / float32(du.Size())
}

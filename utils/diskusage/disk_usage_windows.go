package diskusage

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Statfs returns file system statistics.
func Statfs(volumePath string) (DiskUsage, error) {
	diskUsage, err := newWindowsDiskUsage(volumePath)
	if err != nil {
		return DiskUsage{}, err
	}
	return DiskUsage{
		AvailableBytes: diskUsage.available(),
		SizeBytes:      diskUsage.size(),
	}, nil
}

type windowsDiskUsage struct {
	freeBytes  int64
	totalBytes int64
	availBytes int64
}

// newWindowsDiskUsage returns the disk usage of volumePath, or an error if the path is invalid
// or the syscall fails. Returning an error (rather than a zeroed struct, which callers read as a
// full disk) avoids false low-space warnings and spurious blocks.
func newWindowsDiskUsage(volumePath string) (*windowsDiskUsage, error) {
	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	du := &windowsDiskUsage{}

	utf16Ptr, err := syscall.UTF16PtrFromString(volumePath)
	if err != nil {
		return nil, fmt.Errorf("converting path %q: %w", volumePath, err)
	}

	// GetDiskFreeSpaceExW returns nonzero on success; r1 == 0 means it failed and callErr holds
	// the OS error (callErr is only meaningful when r1 == 0).
	r1, _, callErr := c.Call(
		uintptr(unsafe.Pointer(utf16Ptr)),
		uintptr(unsafe.Pointer(&du.freeBytes)),
		uintptr(unsafe.Pointer(&du.totalBytes)),
		uintptr(unsafe.Pointer(&du.availBytes)))
	if r1 == 0 {
		return nil, fmt.Errorf("GetDiskFreeSpaceExW failed for %q: %w", volumePath, callErr)
	}

	return du, nil
}

// available returns total available bytes on file system to an unprivileged user
func (du *windowsDiskUsage) available() uint64 {
	return uint64(du.availBytes)
}

// size returns total size of the file system
func (du *windowsDiskUsage) size() uint64 {
	return uint64(du.totalBytes)
}

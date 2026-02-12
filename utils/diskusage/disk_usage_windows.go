package diskusage

import (
	"syscall"
	"unsafe"
)

// Statfs returns file system statistics.
func Statfs(volumePath string) (DiskUsage, error) {
	diskUsage := newWindowsDiskUsage(volumePath)
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

// newWindowsDiskUsage returns an object holding the disk usage of volumePath
// or nil in case of error (invalid path, etc)
func newWindowsDiskUsage(volumePath string) *windowsDiskUsage {

	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	du := &windowsDiskUsage{}

	utf16Ptr, err := syscall.UTF16PtrFromString(volumePath)
	if err != nil {
		return nil
	}

	c.Call(
		uintptr(unsafe.Pointer(utf16Ptr)),
		uintptr(unsafe.Pointer(&du.freeBytes)),
		uintptr(unsafe.Pointer(&du.totalBytes)),
		uintptr(unsafe.Pointer(&du.availBytes)))

	return du
}

// available returns total available bytes on file system to an unprivileged user
func (du *windowsDiskUsage) available() uint64 {
	return uint64(du.availBytes)
}

// size returns total size of the file system
func (du *windowsDiskUsage) size() uint64 {
	return uint64(du.totalBytes)
}

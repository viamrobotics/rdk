package diskusage

import (
	"syscall"
	"unsafe"
)

type WindowsDiskUsage struct {
	freeBytes  int64
	totalBytes int64
	availBytes int64
}

// Statfs returns file system statistics.
func Statfs(volumePath string) (DiskUsage, error) {
	diskUsage := NewWindowsDiskUsage(volumePath)
	return DiskUsage{
		AvailableBytes: diskUsage.Available(),
		SizeBytes:      diskUsage.Size(),
	}, nil
}

// NewWindowsDiskUsage returns an object holding the disk usage of volumePath
// or nil in case of error (invalid path, etc)
func NewWindowsDiskUsage(volumePath string) *WindowsDiskUsage {

	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	du := &WindowsDiskUsage{}

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

// Available returns total available bytes on file system to an unprivileged user
func (du *WindowsDiskUsage) Available() uint64 {
	return uint64(du.availBytes)
}

// Size returns total size of the file system
func (du *WindowsDiskUsage) Size() uint64 {
	return uint64(du.totalBytes)
}

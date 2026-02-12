package utils

import (
	"os"
	"syscall"
	"time"
)

// getFileTimes returns the creation and modification times for a file on Windows.
func getFileTimes(info os.FileInfo) (FileTimes, error) {
	modTime := info.ModTime()

	// Get Windows-specific file attributes
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		// If we can't get the Win32FileAttributeData, fall back to using ModTime for both
		return FileTimes{
			CreateTime: modTime,
			ModifyTime: modTime,
		}, nil
	}

	// Convert Windows FILETIME to Go time.Time
	createTime := time.Unix(0, stat.CreationTime.Nanoseconds())

	return FileTimes{
		CreateTime: createTime,
		ModifyTime: modTime,
	}, nil
}

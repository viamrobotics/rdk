//go:build !windows

package utils

import (
	"os"
	"syscall"
)

// getFileTimes returns the creation and modification times for a file.
// On Unix systems, creation time uses:
//   - macOS/BSD: birthtime (Birthtimespec)
//   - Linux: change time (Ctim) as a fallback, since true creation time
//     is not available on all filesystems
func getFileTimes(info os.FileInfo) (FileTimes, error) {
	modTime := info.ModTime()

	// Get platform-specific stat structure
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		// If we can't get the syscall.Stat_t, fall back to using ModTime for both
		return FileTimes{
			CreateTime: modTime,
			ModifyTime: modTime,
		}, nil
	}

	// Get birth time (platform-specific implementation)
	createTime := getBirthTime(stat)

	return FileTimes{
		CreateTime: createTime,
		ModifyTime: modTime,
	}, nil
}

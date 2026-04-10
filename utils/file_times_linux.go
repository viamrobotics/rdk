//go:build linux

package utils

import (
	"syscall"
	"time"
)

// getBirthTime returns the change time (ctime) on Linux as a fallback,
// since true birth time (btime) is not universally available across all filesystems.
func getBirthTime(stat *syscall.Stat_t) time.Time {
	// Cast to int64 for 32-bit compatibility where Ctim fields are int32.
	//nolint:unconvert
	return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
}

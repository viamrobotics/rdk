//go:build linux

package utils

import (
	"syscall"
	"time"
)

// getBirthTime returns the change time (ctime) on Linux as a fallback,
// since true birth time (btime) is not universally available across all filesystems.
func getBirthTime(stat *syscall.Stat_t) time.Time {
	return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}

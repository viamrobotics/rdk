//go:build darwin

package utils

import (
	"syscall"
	"time"
)

// getBirthTime returns the birth time (creation time) on macOS.
func getBirthTime(stat *syscall.Stat_t) time.Time {
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}

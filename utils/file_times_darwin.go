//go:build darwin

package utils

import (
	"syscall"
	"time"
)

// getBirthTime returns the birth time (creation time) on macOS.
func getBirthTime(stat *syscall.Stat_t) time.Time {
	// Cast to int64 for consistency and 32-bit compatibility
	return time.Unix(int64(stat.Birthtimespec.Sec), int64(stat.Birthtimespec.Nsec))
}

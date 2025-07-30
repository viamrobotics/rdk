// Package sys provides functionality for gathering system metrics in an FTDC compliant API.
package sys

import (
	"os"

	"go.viam.com/rdk/ftdc"
)

type stats struct {
	UserCPUSecs     float64
	SystemCPUSecs   float64
	ElapsedTimeSecs float64
	VssMB           float64
	RssMB           float64
}

// NewSysUsageStatser returns a system ftdc statser based on the passed pid.
func NewSysUsageStatser(pid int) (ftdc.Statser, error) {
	return newSysUsageStatser(pid)
}

// NewSelfSysUsageStatser returns a system ftdc statser based on the current pid.
func NewSelfSysUsageStatser() (ftdc.Statser, error) {
	return newSysUsageStatser(os.Getpid())
}

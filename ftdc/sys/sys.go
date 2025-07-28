// Package sys provides functionality for gathering system metrics in an FTDC compliant API.
package sys

import (
	"os"
)

type Statser interface {
	Stats() any
}

type stats struct {
	UserCPUSecs     float64
	SystemCPUSecs   float64
	ElapsedTimeSecs float64
	VssMB           float64
	RssMB           float64
}

func NewSysUsageStatser(pid int) (Statser, error) {
	return newSysUsageStatser(pid)
}

func NewSelfSysUsageStatser() (Statser, error) {
	return newSysUsageStatser(os.Getpid())
}

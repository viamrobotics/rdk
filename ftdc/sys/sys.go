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

func NewUsageStatser(pid int) (Statser, error) {
	return newUsageStatser(pid)
}

func NewSelfUsageStatser() (Statser, error) {
	return newUsageStatser(os.Getpid())
}

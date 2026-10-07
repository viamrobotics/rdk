package sys

import (
	"go.viam.com/rdk/ftdc"
)

// hostCPUStats are shares of all cores' time over the interval between two `Stats` calls.
type hostCPUStats struct {
	UserPct        float64
	SystemPct      float64
	IowaitPct      float64
	IRQPct         float64
	StealPct       float64
	IdlePct        float64
	MaxCoreBusyPct float64
}

// pressureStats are the kernel's 10 second averages from /proc/pressure.
type pressureStats struct {
	CPUSomeAvg10 float64
	IOSomeAvg10  float64
	IOFullAvg10  float64
}

type hostStats struct {
	OnlineCPUs int
	CPU        hostCPUStats
	Load1      float64
	Load5      float64
	Load15     float64
	Pressure   *pressureStats
}

// NewHostUsageStatser returns an ftdc statser for machine-wide CPU usage, load and pressure.
func NewHostUsageStatser() (ftdc.Statser, error) {
	return newHostUsage()
}

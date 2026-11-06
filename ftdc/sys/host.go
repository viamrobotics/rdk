package sys

import (
	"go.viam.com/rdk/ftdc"
)

// hostCPUStats are cumulative CPU-seconds for all online cores from the aggregate "cpu" line in
// /proc/stat (procfs converts jiffies to seconds). Guest time is already counted inside User and
// Nice. Compare successive readings to get per-interval rates.
type hostCPUStats struct {
	UserSecs   float64
	SystemSecs float64
	IowaitSecs float64
	IRQSecs    float64
	StealSecs  float64
	IdleSecs   float64
	// TotalSecs is the sum of all other fields; stored explicitly to simplify ratio computation in
	// the parser without re-summing them.
	TotalSecs float64
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

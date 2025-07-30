//go:build windows

package sys

import (
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	machineBootTimeSecsSinceEpoch float64
)

func init() {
	// Get boot time using gopsutil
	bootTime, err := host.BootTime()
	if err != nil {
		// Fallback - could also use WMI or other methods
		machineBootTimeSecsSinceEpoch = 0
	} else {
		machineBootTimeSecsSinceEpoch = float64(bootTime)
	}
}

// UsageStatser can be used to get system metrics for a process.
type UsageStatser struct {
	proc *process.Process
}

// NewPidSysUsageStatser will return a `SysUsageStatser` for the given process id.
// just leave this one
func newSysUsageStatser(pid int) (*UsageStatser, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}
	return &UsageStatser{proc}, nil
}

// Stats returns Stats.
func (sys *UsageStatser) Stats() any {
	// Get CPU times
	cpuTimes, err := sys.proc.Times()
	if err != nil {
		return stats{}
	}

	// Get memory info
	memInfo, err := sys.proc.MemoryInfo()
	if err != nil {
		return stats{}
	}

	// Get process creation time
	createTime, err := sys.proc.CreateTime()
	if err != nil {
		return stats{}
	}

	// Calculate elapsed time
	elapsedTimeSecs := float64(time.Now().UnixMilli()-createTime) / 1000.0

	return stats{
		UserCPUSecs:     cpuTimes.User,   // Already in seconds
		SystemCPUSecs:   cpuTimes.System, // Already in seconds
		ElapsedTimeSecs: elapsedTimeSecs,
		VssMB:           float64(memInfo.VMS) / 1_000_000.0, // Virtual memory
		RssMB:           float64(memInfo.RSS) / 1_000_000.0, // Resident memory
	}
}

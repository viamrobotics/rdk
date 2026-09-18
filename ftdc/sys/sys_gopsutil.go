//go:build darwin || windows

package sys

import (
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// UsageStatser can be used to get system metrics for a process.
type UsageStatser struct {
	proc *process.Process
}

// newSysUsageStatser will return a `UsageStatser` for the given process id.
func newSysUsageStatser(pid int) (*UsageStatser, error) {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}
	return &UsageStatser{proc}, nil
}

// Stats returns Stats.
func (sys *UsageStatser) Stats() any {
	cpuTimes, err := sys.proc.Times()
	if err != nil {
		return stats{}
	}

	memInfo, err := sys.proc.MemoryInfo()
	if err != nil {
		return stats{}
	}

	// CreateTime returns process creations time, in ms.
	createTime, err := sys.proc.CreateTime()
	if err != nil {
		return stats{}
	}

	elapsedTimeSecs := float64(time.Now().UnixMilli()-createTime) / 1000.0

	return stats{
		UserCPUSecs:     cpuTimes.User,
		SystemCPUSecs:   cpuTimes.System,
		ElapsedTimeSecs: elapsedTimeSecs,
		VssMB:           float64(memInfo.VMS) / 1_000_000.0,
		RssMB:           float64(memInfo.RSS) / 1_000_000.0,
	}
}

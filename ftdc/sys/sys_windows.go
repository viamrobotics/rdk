//go:build windows

package sys

import (
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
)

var (
	osPageSize                    int
	machineBootTimeSecsSinceEpoch float64
)

func init() {
	// Windows API provides RSS in bytes already, so there is no need
	// to get the osPageSize
	//osPageSize = os.Getpagesize()

	bootTime, err := host.BootTime()
	if err != nil {
		// heuristic for failing
		machineBootTimeSecsSinceEpoch = 0
	} else {
		machineBootTimeSecsSinceEpoch = float64(bootTime)
	}
}

// UsageStatser can be used to get system metrics for a process.
type UsageStatser struct {
	proc *process.Process
}

// NewSelfSysUsageStatser will return a `SysUsageStatser` for the current process.
func NewSelfSysUsageStatser() (*UsageStatser, error) {
	pid := int32(os.Getpid())
	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}
	return &UsageStatser{proc}, nil
}

// NewPidSysUsageStatser will return a `SysUsageStatser` for the given process id.
func NewPidSysUsageStatser(pid int) (*UsageStatser, error) {
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

	// Get process creation time, in milliseconds
	createTime, err := sys.proc.CreateTime()
	if err != nil {
		return stats{}
	}

	// elapsedTimeSecs is the time the program started in seconds since the epoch.
	elapsedTimeSecs := float64(time.Now().UnixMilli()-createTime) / 1000.0

	return stats{
		UserCPUSecs:     cpuTimes.User,
		SystemCPUSecs:   cpuTimes.System,
		ElapsedTimeSecs: elapsedTimeSecs,
		// memInfo is already in bytes
		VssMB: float64(memInfo.VMS) / 1_000_000.0,
		RssMB: float64(memInfo.RSS) / 1_000_000.0,
	}
}

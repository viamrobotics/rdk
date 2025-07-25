//go:build windows

package sys

import (
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/process"
	"go.viam.com/rdk/logging"
)

var (
	osPageSize                    int
	machineBootTimeSecsSinceEpoch float64
)

func init() {
	// windows doesn't need pageSize because RSS is in bytes already
	osPageSize = os.Getpagesize()

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
	proc   *process.Process
	logger logging.Logger
}

// NewSelfSysUsageStatser will return a `SysUsageStatser` for the current process.
func NewSelfSysUsageStatser(logger logging.Logger) (*UsageStatser, error) {
	usageLogger := logger.Sublogger("sys-metrics-windows")
	usageLogger.NeverDeduplicate()
	pid := int32(os.Getpid())
	proc, err := process.NewProcess(pid)
	if err != nil {
		usageLogger.Warn(err)
		return nil, err
	}
	return &UsageStatser{proc, usageLogger}, nil
}

// NewPidSysUsageStatser will return a `SysUsageStatser` for the given process id.
func NewPidSysUsageStatser(pid int, logger logging.Logger) (*UsageStatser, error) {
	usageLogger := logger.Sublogger("sys-metrics-windows-mod")
	usageLogger.NeverDeduplicate()
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil, err
	}
	return &UsageStatser{proc, usageLogger}, nil
}

// Stats returns Stats.
func (sys *UsageStatser) Stats() any {
	// Get CPU times
	cpuTimes, err := sys.proc.Times()
	sys.logger.Info("error 1", err)
	if err != nil {
		return stats{}
	}

	// Get memory info
	memInfo, err := sys.proc.MemoryInfo()
	sys.logger.Info("error 2", err)
	if err != nil {
		return stats{}
	}

	// Get process creation time, in ms
	createTime, err := sys.proc.CreateTime()
	sys.logger.Info("error 3", err)
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

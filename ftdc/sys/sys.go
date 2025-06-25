// Package sys provides functionality for gathering system metrics in an FTDC compliant API.
package sys

import (
	"os"
	"time"

	"github.com/prometheus/procfs"
)

// On linux, getting the page size is a system call. Cache the page size for the entirety of the
// program lifetime. As opposed to calling it each time we wish to compute the resident memory a
// program is using.
var (
	osPageSize                    int
	machineBootTimeSecsSinceEpoch float64
)

func init() {
	osPageSize = os.Getpagesize()

	machine, err := procfs.NewDefaultFS()
	if err != nil {
		return
	}

	machineStats, err := machine.Stat()
	if err != nil {
		return
	}

	machineBootTimeSecsSinceEpoch = float64(machineStats.BootTime)
}

// UsageStatser can be used to get system metrics for a process.
type UsageStatser struct {
	proc procfs.Proc
}

// NewSelfSysUsageStatser will return a `SysUsageStatser` for the current process.
func NewSelfSysUsageStatser() (*UsageStatser, error) {
	process, err := procfs.Self()
	if err != nil {
		return nil, err
	}

	return &UsageStatser{process}, nil
}

// NewPidSysUsageStatser will return a `SysUsageStatser` for the given process id.
func NewPidSysUsageStatser(pid int) (*UsageStatser, error) {
	process, err := procfs.NewProc(pid)
	if err != nil {
		return nil, err
	}

	return &UsageStatser{process}, nil
}

type stats struct {
	UserCPUSecs     float64
	SystemCPUSecs   float64
	ElapsedTimeSecs float64
	VssMB           float64
	RssMB           float64
}

// Stats returns Stats.
func (sys *UsageStatser) Stats() any {
	// Stats files refer to time in "clock ticks". The right way to learn of the tick time (on
	// linux) is via a system call to `sysconf(_SC_CLK_TCK)`. That system call, however, requires
	// cgo. And it's almost universally true that 100hz is the configured value for "modern"
	// systems.
	//
	// We should feel empowered to revisit this decision if the above assumption is not true. It's
	// important to have the right value such that we can compute the amount of time a program has
	// been running accurately. Without that, computing metrics like CPU percentage are incorrect.
	const userHz = 100

	stat, err := sys.proc.Stat()
	if err != nil {
		return stats{}
	}

	// relativeStartTimeSecs is the time the program started in seconds since the machine was
	// booted.
	relativeStartTimeSecs := float64(stat.Starttime) / float64(userHz)

	// absoluteStartTimeSecs is the time the program started in seconds since the epoch.
	absoluteStartTimeSecs := machineBootTimeSecsSinceEpoch + relativeStartTimeSecs

	const nanosPerSecond = float64(1_000_000_000)
	return stats{
		UserCPUSecs:     float64(stat.UTime) / float64(userHz),
		SystemCPUSecs:   float64(stat.STime) / float64(userHz),
		ElapsedTimeSecs: float64(time.Now().UnixNano())/nanosPerSecond - absoluteStartTimeSecs,
		VssMB:           float64(stat.VSize) / 1_000_000.0,
		RssMB:           float64(stat.RSS*osPageSize) / 1_000_000.0,
	}
}

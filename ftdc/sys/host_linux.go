//go:build linux

package sys

import (
	"github.com/prometheus/procfs"
)

type hostStatser struct {
	fs     procfs.FS
	hasPSI bool
}

func newHostUsage() (*hostStatser, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}
	return newHostStatser(fs)
}

func newHostStatser(fs procfs.FS) (*hostStatser, error) {
	if _, err := fs.Stat(); err != nil {
		return nil, err
	}
	_, psiErr := fs.PSIStatsForResource("cpu")
	return &hostStatser{fs: fs, hasPSI: psiErr == nil}, nil
}

func (h *hostStatser) Stats() any {
	var ret hostStats
	if load, err := h.fs.LoadAvg(); err == nil {
		ret.Load1, ret.Load5, ret.Load15 = load.Load1, load.Load5, load.Load15
	}
	if h.hasPSI {
		ret.Pressure = &pressureStats{}
		if cpu, err := h.fs.PSIStatsForResource("cpu"); err == nil && cpu.Some != nil {
			ret.Pressure.CPUSomeAvg10 = cpu.Some.Avg10
		}
		if io, err := h.fs.PSIStatsForResource("io"); err == nil {
			if io.Some != nil {
				ret.Pressure.IOSomeAvg10 = io.Some.Avg10
			}
			if io.Full != nil {
				ret.Pressure.IOFullAvg10 = io.Full.Avg10
			}
		}
	}

	stat, err := h.fs.Stat()
	if err != nil {
		return ret
	}
	ret.OnlineCPUs = len(stat.CPU)
	c := stat.CPUTotal
	user := c.User + c.Nice
	system := c.System
	iowait := c.Iowait
	irq := c.IRQ + c.SoftIRQ
	steal := c.Steal
	idle := c.Idle
	ret.CPU = hostCPUStats{
		UserSecs:   user,
		SystemSecs: system,
		IowaitSecs: iowait,
		IRQSecs:    irq,
		StealSecs:  steal,
		IdleSecs:   idle,
		TotalSecs:  user + system + iowait + irq + steal + idle,
	}
	return ret
}

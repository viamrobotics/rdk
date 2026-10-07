//go:build linux

package sys

import (
	"github.com/prometheus/procfs"
)

type hostStatser struct {
	fs procfs.FS
	// hasPSI is fixed at construction so the `Pressure` section is either always or never present.
	hasPSI bool

	prevTotal procfs.CPUStat
	prevCores map[int64]procfs.CPUStat
}

func newHostUsage() (*hostStatser, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}
	return newHostStatser(fs)
}

func newHostStatser(fs procfs.FS) (*hostStatser, error) {
	stat, err := fs.Stat()
	if err != nil {
		return nil, err
	}
	_, psiErr := fs.PSIStatsForResource("cpu")
	return &hostStatser{fs: fs, hasPSI: psiErr == nil, prevTotal: stat.CPUTotal, prevCores: stat.CPU}, nil
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
	ret.CPU, _ = cpuShares(h.prevTotal, stat.CPUTotal)
	for id, core := range stat.CPU {
		// A core that just came online has no previous sample to diff against.
		prev, seen := h.prevCores[id]
		if !seen {
			continue
		}
		if shares, ok := cpuShares(prev, core); ok {
			ret.CPU.MaxCoreBusyPct = max(ret.CPU.MaxCoreBusyPct, 100-shares.IdlePct-shares.IowaitPct)
		}
	}
	h.prevTotal, h.prevCores = stat.CPUTotal, stat.CPU
	return ret
}

// cpuShares returns how `cur - prev` splits across states, as percentages, and false when no time
// elapsed. /proc/stat already counts guest time inside user and nice.
func cpuShares(prev, cur procfs.CPUStat) (hostCPUStats, bool) {
	delta := func(p, c float64) float64 { return max(c-p, 0) }
	user := delta(prev.User+prev.Nice, cur.User+cur.Nice)
	system := delta(prev.System, cur.System)
	iowait := delta(prev.Iowait, cur.Iowait)
	irq := delta(prev.IRQ+prev.SoftIRQ, cur.IRQ+cur.SoftIRQ)
	steal := delta(prev.Steal, cur.Steal)
	idle := delta(prev.Idle, cur.Idle)

	total := user + system + iowait + irq + steal + idle
	if total == 0 {
		return hostCPUStats{}, false
	}
	pct := func(v float64) float64 { return 100 * v / total }
	return hostCPUStats{
		UserPct:   pct(user),
		SystemPct: pct(system),
		IowaitPct: pct(iowait),
		IRQPct:    pct(irq),
		StealPct:  pct(steal),
		IdlePct:   pct(idle),
	}, true
}

package sys

import (
	"github.com/prometheus/procfs"
)

type netStatser struct {
	fs procfs.FS
}

// NewNetUsage returns an object that can interpreted as an `ftdc.Statser`.
//
//nolint:revive
func NewNetUsage() (*netStatser, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	return &netStatser{fs}, nil
}

type netDevLine struct {
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	RxDropped uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
	TxDropped uint64
}

type ifaceStats struct {
	TxQueueLength uint64
	RxQueueLength uint64
	UsedSockets   uint64
	Drops         uint64
}

type networkStats struct {
	Ifaces map[string]netDevLine
	TCP    ifaceStats
	UDP    ifaceStats
}

func (netStatser *netStatser) Stats() any {
	ret := networkStats{
		Ifaces: make(map[string]netDevLine),
	}
	if dev, err := netStatser.fs.NetDev(); err == nil {
		for ifaceName, stats := range dev {
			ret.Ifaces[ifaceName] = netDevLine{
				stats.RxBytes, stats.RxPackets, stats.RxErrors, stats.RxDropped,
				stats.TxBytes, stats.TxPackets, stats.TxErrors, stats.TxDropped,
			}
		}
	}

	if netTCPSummary, err := netStatser.fs.NetTCPSummary(); err == nil {
		ret.TCP.TxQueueLength = netTCPSummary.TxQueueLength
		ret.TCP.RxQueueLength = netTCPSummary.RxQueueLength
		ret.TCP.UsedSockets = netTCPSummary.UsedSockets
	}

	if netUDPSummary, err := netStatser.fs.NetUDPSummary(); err == nil {
		ret.UDP.TxQueueLength = netUDPSummary.TxQueueLength
		ret.UDP.RxQueueLength = netUDPSummary.RxQueueLength
		ret.UDP.UsedSockets = netUDPSummary.UsedSockets
		ret.UDP.Drops = *netUDPSummary.Drops
	}

	return ret
}

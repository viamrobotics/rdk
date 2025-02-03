package sys

import (
	"github.com/prometheus/procfs"
)

type NetStatser struct {
	fs procfs.FS
}

func NewNetUsage() (*NetStatser, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	return &NetStatser{fs}, nil
}

type NetDevLine struct {
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	RxDropped uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
	TxDropped uint64
}

type IfaceStats struct {
	TxQueueLength uint64
	RxQueueLength uint64
	UsedSockets   uint64
	Drops         uint64
}

type NetworkStats struct {
	Ifaces map[string]NetDevLine
	TCP    IfaceStats
	UDP    IfaceStats
}

func (netStatser *NetStatser) Stats() any {
	ret := NetworkStats{
		Ifaces: make(map[string]NetDevLine),
	}
	if dev, err := netStatser.fs.NetDev(); err == nil {
		for ifaceName, stats := range dev {
			ret.Ifaces[ifaceName] = NetDevLine{
				stats.RxBytes, stats.RxPackets, stats.RxErrors, stats.RxDropped,
				stats.TxBytes, stats.TxPackets, stats.TxErrors, stats.TxDropped,
			}
		}
	}

	if netTcpSummary, err := netStatser.fs.NetTCPSummary(); err == nil {
		ret.TCP.TxQueueLength = netTcpSummary.TxQueueLength
		ret.TCP.RxQueueLength = netTcpSummary.RxQueueLength
		ret.TCP.UsedSockets = netTcpSummary.UsedSockets
	}

	if netUdpSummary, err := netStatser.fs.NetUDPSummary(); err == nil {
		ret.UDP.TxQueueLength = netUdpSummary.TxQueueLength
		ret.UDP.RxQueueLength = netUdpSummary.RxQueueLength
		ret.UDP.UsedSockets = netUdpSummary.UsedSockets
		ret.UDP.Drops = *netUdpSummary.Drops
	}

	return ret
}

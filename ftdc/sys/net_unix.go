//go:build unix

package sys

import (
	"github.com/prometheus/procfs"
)

type netStatser struct {
	fs procfs.FS
}

// NewNetUsage returns an object that can interpreted as an `ftdc.Statser`.
func newNetUsage() (*netStatser, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	return &netStatser{fs}, nil
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
		if netUDPSummary.Drops != nil {
			ret.UDP.Drops = *netUDPSummary.Drops
		}
	}

	return ret
}

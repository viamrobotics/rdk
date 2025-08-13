//go:build windows

package sys

import (
	windows_net "github.com/shirou/gopsutil/v3/net"
)

type netStatser struct{}

// NewNetUsage returns an object that can interpreted as an `ftdc.Statser`.
func newNetUsage() (*netStatser, error) {
	return &netStatser{}, nil
}
func (netStatser *netStatser) Stats() any {
	ret := networkStats{
		Ifaces: make(map[string]netDevLine),
	}

	if netIOs, err := windows_net.IOCounters(true); err == nil {
		for _, stat := range netIOs {
			ret.Ifaces[stat.Name] = netDevLine{
				RxBytes:   stat.BytesRecv,
				RxPackets: stat.PacketsRecv,
				RxErrors:  stat.Errin,
				RxDropped: stat.Dropin,
				TxBytes:   stat.BytesSent,
				TxPackets: stat.PacketsSent,
				TxErrors:  stat.Errout,
				TxDropped: stat.Dropout,
			}
		}
	}

	if conns, err := windows_net.Connections("tcp"); err == nil {
		ret.TCP.UsedSockets = uint64(len(conns))
		// Note: TX/RX queue lengths not exposed here
	}

	if conns, err := windows_net.Connections("udp"); err == nil {
		ret.UDP.UsedSockets = uint64(len(conns))
	}

	return ret
}

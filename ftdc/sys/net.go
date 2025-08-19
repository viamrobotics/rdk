package sys

import (
	"go.viam.com/rdk/ftdc"
)

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

// NewNetUsageStatser returns a network ftdc statser.
func NewNetUsageStatser() (ftdc.Statser, error) {
	return newNetUsage()
}

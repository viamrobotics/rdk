//go:build windows

package sys

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type netStatser struct {
	iphlpapi *windows.LazyDLL
}

// NewNetUsage returns an object that can interpreted as an `ftdc.Statser`.
func NewNetUsage() (*netStatser, error) {
	return &netStatser{
		iphlpapi: windows.NewLazySystemDLL("iphlpapi.dll"),
	}, nil
}

// MIB_IFROW structure for GetIfTable
type mibIfRow struct {
	Name            [256]uint16
	Index           uint32
	Type            uint32
	Mtu             uint32
	Speed           uint32
	PhysAddrLen     uint32
	PhysAddr        [8]byte
	AdminStatus     uint32
	OperStatus      uint32
	LastChange      uint32
	InOctets        uint32
	InUcastPkts     uint32
	InNUcastPkts    uint32
	InDiscards      uint32
	InErrors        uint32
	InUnknownProtos uint32
	OutOctets       uint32
	OutUcastPkts    uint32
	OutNUcastPkts   uint32
	OutDiscards     uint32
	OutErrors       uint32
	OutQLen         uint32
	DescrLen        uint32
	Descr           [256]byte
}

// TCP connection stats from GetTcpStatistics
type mibTcpStats struct {
	RtoAlgorithm uint32
	RtoMin       uint32
	RtoMax       uint32
	MaxConn      uint32
	ActiveOpens  uint32
	PassiveOpens uint32
	AttemptFails uint32
	EstabResets  uint32
	CurrEstab    uint32
	InSegs       uint32
	OutSegs      uint32
	RetransSegs  uint32
	InErrs       uint32
	OutRsts      uint32
}

// UDP stats from GetUdpStatistics
type mibUdpStats struct {
	InDatagrams  uint32
	NoPorts      uint32
	InErrors     uint32
	OutDatagrams uint32
	NumAddrs     uint32
}

func (n *netStatser) Stats() any {
	ret := networkStats{
		Ifaces: make(map[string]netDevLine),
	}

	// Get network interface statistics using GetIfTable
	n.getInterfaceStats(&ret)

	// Get TCP statistics
	n.getTCPStats(&ret)

	// Get UDP statistics
	n.getUDPStats(&ret)

	return ret
}

func (n *netStatser) getInterfaceStats(ret *networkStats) {
	getIfTable := n.iphlpapi.NewProc("GetIfTable")

	var size uint32
	// First call to get required buffer size
	getIfTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)

	if size == 0 {
		return
	}

	buffer := make([]byte, size)
	r1, _, _ := getIfTable.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
	)

	if r1 != 0 { // NO_ERROR is 0
		return
	}

	// Parse the MIB_IFTABLE structure
	numEntries := *(*uint32)(unsafe.Pointer(&buffer[0]))
	offset := unsafe.Sizeof(uint32(0))

	for i := uint32(0); i < numEntries; i++ {
		row := (*mibIfRow)(unsafe.Pointer(&buffer[offset]))

		// Convert interface name from UTF-16
		name := windows.UTF16ToString(row.Name[:])
		if name == "" {
			// Fallback to description if name is empty
			name = string(row.Descr[:row.DescrLen])
		}

		ret.Ifaces[name] = netDevLine{
			RxBytes:   uint64(row.InOctets),
			RxPackets: uint64(row.InUcastPkts + row.InNUcastPkts),
			RxErrors:  uint64(row.InErrors),
			RxDropped: uint64(row.InDiscards),
			TxBytes:   uint64(row.OutOctets),
			TxPackets: uint64(row.OutUcastPkts + row.OutNUcastPkts),
			TxErrors:  uint64(row.OutErrors),
			TxDropped: uint64(row.OutDiscards),
		}

		offset += unsafe.Sizeof(mibIfRow{})
	}
}

func (n *netStatser) getTCPStats(ret *networkStats) {
	getTcpStatistics := n.iphlpapi.NewProc("GetTcpStatistics")

	var stats mibTcpStats
	r1, _, _ := getTcpStatistics.Call(uintptr(unsafe.Pointer(&stats)))

	if r1 == 0 { // NO_ERROR
		ret.TCP.UsedSockets = uint64(stats.CurrEstab)
		// Windows doesn't directly expose TX/RX queue lengths like Linux /proc
		// These values would need additional API calls to GetTcpTable for per-connection data
		ret.TCP.TxQueueLength = 0 // Not directly available
		ret.TCP.RxQueueLength = 0 // Not directly available
	}
}

func (n *netStatser) getUDPStats(ret *networkStats) {
	getUdpStatistics := n.iphlpapi.NewProc("GetUdpStatistics")

	var stats mibUdpStats
	r1, _, _ := getUdpStatistics.Call(uintptr(unsafe.Pointer(&stats)))

	if r1 == 0 { // NO_ERROR
		ret.UDP.UsedSockets = uint64(stats.NumAddrs)
		ret.UDP.Drops = uint64(stats.InErrors)
		// Windows doesn't directly expose TX/RX queue lengths like Linux /proc
		ret.UDP.TxQueueLength = 0 // Not directly available
		ret.UDP.RxQueueLength = 0 // Not directly available
	}
}

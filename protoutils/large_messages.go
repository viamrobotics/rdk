package protoutils

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
)

// LargeMessageSite names a call site that LargeMessageStatser reports on.
type LargeMessageSite string

// Sites that convert arbitrary Go values to structpb, where []byte inflates ~11x.
const (
	SiteDoCommandServer LargeMessageSite = "docommand_server"
	SiteDoCommandClient LargeMessageSite = "docommand_client"
	SiteTabularCapture  LargeMessageSite = "tabular_capture"
)

const (
	size1MB = 1 << 20
	size5MB = 5 << 20

	// minByteListLen keeps short whole-number lists (pins, joint indices, RGB) from counting as byte
	// arrays. A byte list big enough to push a message past 1 MiB has ~95k entries, so this misses
	// nothing that matters.
	minByteListLen = 1024
)

type largeMessageCounters struct {
	over1MB          atomic.Int64
	over5MB          atomic.Int64
	byteListsOver1MB atomic.Int64
	byteListsOver5MB atomic.Int64
}

var (
	largeMessages = map[LargeMessageSite]*largeMessageCounters{
		SiteDoCommandServer: {},
		SiteDoCommandClient: {},
		SiteTabularCapture:  {},
	}
	largeMessageLogger atomic.Pointer[logging.Logger]
	largeMessageWarned sync.Map
)

// SetLargeMessageLogger sets the logger RecordMessageSize warns on. Without one, it only counts.
func SetLargeMessageLogger(logger logging.Logger) {
	largeMessageLogger.Store(&logger)
}

// RecordMessageSize counts m toward site if it is at least 1 MiB or 5 MiB on the wire, separately
// counting those that contain a []byte encoded as a list of numbers, and warns the first time
// each name crosses each size.
func RecordMessageSize(site LargeMessageSite, name string, m *structpb.Struct) {
	size := proto.Size(m)
	// Only walk messages already known to be large; small ones pay just for proto.Size.
	if size < size1MB {
		return
	}
	byteListBytes := countByteLists(m)
	c := largeMessages[site]
	c.over1MB.Add(1)
	if byteListBytes > 0 {
		c.byteListsOver1MB.Add(1)
	}
	threshold := "1MB"
	if size >= size5MB {
		c.over5MB.Add(1)
		if byteListBytes > 0 {
			c.byteListsOver5MB.Add(1)
		}
		threshold = "5MB"
	}

	logger := largeMessageLogger.Load()
	if logger == nil {
		return
	}
	if _, seen := largeMessageWarned.LoadOrStore(fmt.Sprintf("%s/%s/%s", site, name, threshold), struct{}{}); seen {
		return
	}
	(*logger).Warnw("large message",
		"site", site, "name", name, "size", size, "threshold", threshold, "byte_list_bytes", byteListBytes)
}

// countByteLists returns the total length of lists in s that look like a []byte encoded one number
// per byte. go.viam.com/utils/protoutils.StructToStructPb encodes []byte this way, and no type
// information survives, so detection is by shape: a long list of whole numbers in [0, 255]. Long
// lists of small ints (pixels, histograms) also match, but they inflate the same way, so a positive
// here is not that bad.
func countByteLists(s *structpb.Struct) int {
	var n int
	var walk func(*structpb.Value)
	walk = func(v *structpb.Value) {
		switch k := v.GetKind().(type) {
		case *structpb.Value_StructValue:
			for _, f := range k.StructValue.GetFields() {
				walk(f)
			}
		case *structpb.Value_ListValue:
			vals := k.ListValue.GetValues()
			if len(vals) >= minByteListLen && allBytes(vals) {
				n += len(vals)
				return
			}
			// Byte arrays can sit inside lists of maps.
			for _, e := range vals {
				walk(e)
			}
		}
	}
	for _, f := range s.GetFields() {
		walk(f)
	}
	return n
}

func allBytes(vals []*structpb.Value) bool {
	for _, v := range vals {
		n, ok := v.GetKind().(*structpb.Value_NumberValue)
		if !ok || n.NumberValue < 0 || n.NumberValue > 255 || n.NumberValue != math.Trunc(n.NumberValue) {
			return false
		}
	}
	return true
}

// LargeMessageStats holds cumulative counts of messages at or above each size.
type LargeMessageStats struct {
	Over1MB          int64
	Over5MB          int64
	ByteListsOver1MB int64
	ByteListsOver5MB int64
}

// LargeMessageStatser reports per-site large message counts to FTDC.
type LargeMessageStatser struct{}

// Stats satisfies ftdc.Statser.
func (LargeMessageStatser) Stats() any {
	ret := make(map[string]LargeMessageStats, len(largeMessages))
	for site, c := range largeMessages {
		ret[string(site)] = LargeMessageStats{
			Over1MB:          c.over1MB.Load(),
			Over5MB:          c.over5MB.Load(),
			ByteListsOver1MB: c.byteListsOver1MB.Load(),
			ByteListsOver5MB: c.byteListsOver5MB.Load(),
		}
	}
	return ret
}

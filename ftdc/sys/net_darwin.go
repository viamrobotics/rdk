//go:build darwin

package sys

import (
	"errors"

	"go.viam.com/rdk/ftdc"
)

// Network stats are not collected on darwin. gopsutil's darwin implementation shells out to
// `netstat -ibdnW` for interface counters and to `lsof` for the socket summaries, which measured
// at roughly 3.5ms and 23ms per call respectively. `Stats` runs once a second, so collecting
// these would spend ~50ms of every second forking subprocesses to gather metrics that are only
// consulted for deployed linux robots.
func newNetUsage() (ftdc.Statser, error) {
	return nil, errors.New("network stats are not collected on darwin")
}

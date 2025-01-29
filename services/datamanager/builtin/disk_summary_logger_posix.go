//go:build !windows

package builtin

import "go.viam.com/rdk/utils/diskusage"

func (poller *diskSummaryLogger) logDiskUsage(dir string) {
	poller.logger.Debug(diskusage.Statfs(dir))
}

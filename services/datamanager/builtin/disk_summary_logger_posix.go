//go:build linux || darwin

package builtin

import "go.viam.com/rdk/utils/diskusage"

func (poller *diskSummaryLogger) logDiskUsage(dir string) {
	poller.logger.Info(diskusage.Statfs(dir))
}

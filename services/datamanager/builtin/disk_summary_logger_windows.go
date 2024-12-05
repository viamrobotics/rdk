package builtin

func (poller *diskSummaryLogger) logDiskUsage(dir string) {
	poller.logger.Warn("can't log disk usage yet on windows")
}

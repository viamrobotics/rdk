package builtin

import (
	"context"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
)

// diskSummaryLogger logs a summary of the capture directory and additional
// sync paths for observability.
type diskSummaryLogger struct {
	logger logging.Logger
	worker *goutils.StoppableWorkers
}

func newDiskSummaryLogger(logger logging.Logger) *diskSummaryLogger {
	return &diskSummaryLogger{
		logger: logger,
		worker: goutils.NewBackgroundStoppableWorkers(),
	}
}

func (poller *diskSummaryLogger) reconfigure(dirs []string, interval time.Duration) {
	poller.worker.Stop()
	poller.worker = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
		poller.logger.Debug("datamanager disk state summary logger starting...")
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			if ctx.Err() != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-t.C:
				var summary []DirSummary
				for _, dir := range dirs {
					summary = append(summary, DiskSummary(ctx, dir)...)
					if ctx.Err() != nil {
						return
					}
				}

				for i, s := range summary {
					if i == 0 {
						poller.logger.Debug("datamanager disk state summary:")
						poller.logDiskUsage(dirs[i])
					}
					var (
						dataTimeRange string
						dataStart     string
						dataEnd       string
					)
					if s.DataTimeRange != nil {
						dtr := *s.DataTimeRange
						dataTimeRange = dtr.End.Sub(dtr.Start).Round(time.Second).String()
						dataStart = dtr.Start.String()
						dataEnd = dtr.End.String()
					}
					poller.logger.Debugw(s.Path,
						"file_count", s.FileCount,
						"file_size", data.FormatBytesI64(s.FileSize),
						"data_time_range", dataTimeRange,
						"data_start", dataStart,
						"data_end", dataEnd,
						"err", s.Err)
				}
			}
		}
	})
}

func (poller *diskSummaryLogger) close() {
	poller.worker.Stop()
}

package sync

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
)

type atomicUploadStats struct {
	binary    atomicStat
	tabular   atomicStat
	arbitrary atomicStat
}

type atomicStat struct {
	uploadedFileCount     atomic.Uint64
	uploadedBytes         atomic.Uint64
	uploadFailedFileCount atomic.Uint64
}

type uploadStats struct {
	binary    stat
	tabular   stat
	arbitrary stat
}

type stat struct {
	uploadedFileCount     uint64
	uploadedBytes         uint64
	uploadFailedFileCount uint64
}
type statsWorker struct {
	worker *goutils.StoppableWorkers
	logger logging.Logger
}

func newStatsWorker(logger logging.Logger) *statsWorker {
	return &statsWorker{goutils.NewBackgroundStoppableWorkers(), logger}
}

func (sw *statsWorker) reconfigure(atomicStats *atomicUploadStats, interval time.Duration) {
	sw.worker.Stop()
	sw.worker = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
		oldState := newUploadStats(atomicStats)
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
				newState := newUploadStats(atomicStats)
				summary := summary(oldState, newState, interval)
				for _, line := range summary {
					sw.logger.Info(line)
				}
				oldState = newState
			}
		}
	})
}

func newStat(s *atomicStat) stat {
	return stat{
		uploadedFileCount:     s.uploadedFileCount.Load(),
		uploadFailedFileCount: s.uploadFailedFileCount.Load(),
	}
}

func newUploadStats(stats *atomicUploadStats) uploadStats {
	return uploadStats{
		binary:    newStat(&stats.binary),
		tabular:   newStat(&stats.tabular),
		arbitrary: newStat(&stats.arbitrary),
	}
}

func perSecond(v uint64, interval time.Duration) float64 {
	return float64(v) / interval.Seconds()
}

func totalSummary(oldState, newState uploadStats, interval time.Duration) []string {
	sumFileCount := newState.arbitrary.uploadedFileCount + newState.binary.uploadedFileCount + newState.tabular.uploadedFileCount
	aFileCoundDiff := newState.arbitrary.uploadedFileCount - oldState.arbitrary.uploadedFileCount
	bFileCoundDiff := newState.binary.uploadedFileCount - oldState.binary.uploadedFileCount
	tFileCoundDiff := newState.tabular.uploadedFileCount - oldState.tabular.uploadedFileCount
	sumDiffFileCount := aFileCoundDiff + bFileCoundDiff + tFileCoundDiff

	sumFailedCount := newState.arbitrary.uploadFailedFileCount + newState.binary.uploadFailedFileCount + newState.tabular.uploadFailedFileCount
	aFailedFileCoundDiff := newState.arbitrary.uploadFailedFileCount - oldState.arbitrary.uploadFailedFileCount
	bFailedFileCoundDiff := newState.binary.uploadFailedFileCount - oldState.binary.uploadFailedFileCount
	tFailedFileCoundDiff := newState.tabular.uploadFailedFileCount - oldState.tabular.uploadFailedFileCount
	sumDiffFailedFileCount := aFailedFileCoundDiff + bFailedFileCoundDiff + tFailedFileCoundDiff

	return []string{
		fmt.Sprintf("total uploads: %d, rate: %f/sec", sumFileCount, perSecond(sumDiffFileCount, interval)),
		fmt.Sprintf("total failed uploads: %d, rate: %f/sec", sumFailedCount, perSecond(sumDiffFailedFileCount, interval)),
	}
}

func summary(oldState, newState uploadStats, interval time.Duration) []string {
	summary := totalSummary(oldState, newState, interval)
	var empty stat
	if oldState.arbitrary != empty && newState.arbitrary != empty {
		successRate := perSecond(newState.arbitrary.uploadedFileCount-oldState.arbitrary.uploadedFileCount, interval)
		successByteRate := perSecond(newState.arbitrary.uploadedBytes-oldState.arbitrary.uploadedBytes, interval)
		failRate := perSecond(newState.arbitrary.uploadFailedFileCount-oldState.arbitrary.uploadFailedFileCount, interval)
		summary = append(summary, fmt.Sprintf("arbitrary file, (files uploaded): total: %d, rate: %f/sec",
			newState.arbitrary.uploadedFileCount, successRate))
		summary = append(summary, fmt.Sprintf("arbitrary file, (bytes uploaded): total: %d, rate: %f/sec",
			newState.arbitrary.uploadedBytes, successByteRate))
		summary = append(summary, fmt.Sprintf("arbitrary file, (failed file uploads): total: %d, rate: %f/sec",
			newState.arbitrary.uploadFailedFileCount, failRate))
	}

	if oldState.binary != empty && newState.binary != empty {
		successRate := perSecond(newState.binary.uploadedFileCount-oldState.binary.uploadedFileCount, interval)
		successByteRate := perSecond(newState.binary.uploadedBytes-oldState.binary.uploadedBytes, interval)
		failRate := perSecond(newState.binary.uploadFailedFileCount-oldState.binary.uploadFailedFileCount, interval)
		summary = append(summary, fmt.Sprintf("binary file, (files uploaded): total: %d, rate: %f/sec",
			newState.binary.uploadedFileCount, successRate))
		summary = append(summary, fmt.Sprintf("binary file, (bytes uploaded): total: %d, rate: %f/sec",
			newState.binary.uploadedBytes, successByteRate))
		summary = append(summary, fmt.Sprintf("binary file, (failed file uploads): total: %d, rate: %f/sec",
			newState.binary.uploadFailedFileCount, failRate))
	}

	if oldState.tabular != empty && newState.tabular != empty {
		successRate := perSecond(newState.tabular.uploadedFileCount-oldState.tabular.uploadedFileCount, interval)
		successByteRate := perSecond(newState.tabular.uploadedBytes-oldState.tabular.uploadedBytes, interval)
		failRate := perSecond(newState.tabular.uploadFailedFileCount-oldState.tabular.uploadFailedFileCount, interval)
		summary = append(summary, fmt.Sprintf("tabular file, (files uploaded): total: %d, rate: %f/sec",
			newState.tabular.uploadedFileCount, successRate))
		summary = append(summary, fmt.Sprintf("tabular file, (bytes uploaded): total: %d, rate: %f/sec",
			newState.tabular.uploadedBytes, successByteRate))
		summary = append(summary, fmt.Sprintf("tabular file, (failed file uploads): total: %d, rate: %f/sec",
			newState.tabular.uploadFailedFileCount, failRate))
	}

	return summary
}

func (sw *statsWorker) close() {
	sw.worker.Stop()
}

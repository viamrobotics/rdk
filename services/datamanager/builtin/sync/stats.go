package sync

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/data"
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

func (sw *statsWorker) reconfigure(aus *atomicUploadStats, interval time.Duration) {
	sw.worker.Stop()
	sw.worker = goutils.NewBackgroundStoppableWorkers(func(ctx context.Context) {
		oldState := newUploadStats(aus)
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
				newState := newUploadStats(aus)
				summary := summary(oldState, newState, interval)
				for _, line := range summary {
					sw.logger.Debug(line)
				}
				oldState = newState
			}
		}
	})
}

func (sw *statsWorker) close() {
	sw.worker.Stop()
}

func newUploadStats(stats *atomicUploadStats) uploadStats {
	return uploadStats{
		binary:    newStat(&stats.binary),
		tabular:   newStat(&stats.tabular),
		arbitrary: newStat(&stats.arbitrary),
	}
}

func newStat(s *atomicStat) stat {
	return stat{
		uploadedFileCount:     s.uploadedFileCount.Load(),
		uploadedBytes:         s.uploadedBytes.Load(),
		uploadFailedFileCount: s.uploadFailedFileCount.Load(),
	}
}

func perSecond(v uint64, interval time.Duration) float64 {
	return float64(v) / interval.Seconds()
}

func summary(oldState, newState uploadStats, interval time.Duration) []string {
	summary := totalSummary(oldState, newState, interval)
	summary = summarizeStat(summary, "arbitrary", oldState.arbitrary, newState.arbitrary, interval)
	summary = summarizeStat(summary, "binary", oldState.binary, newState.binary, interval)
	summary = summarizeStat(summary, "tabular", oldState.tabular, newState.tabular, interval)
	return summary
}

func totalCountDiff(oldA, oldB, oldT, newA, newB, newT uint64, interval time.Duration) (uint64, string) {
	newSum := newA + newB + newT
	aDiff := newA - oldA
	bDiff := newB - oldB
	tDiff := newT - oldT
	diffSum := aDiff + bDiff + tDiff

	return newSum, fmt.Sprintf("%.2f", perSecond(diffSum, interval))
}

func totalBytesDiff(oldA, oldB, oldT, newA, newB, newT uint64, interval time.Duration) (string, string) {
	newSum := newA + newB + newT
	aDiff := newA - oldA
	bDiff := newB - oldB
	tDiff := newT - oldT
	diffSum := aDiff + bDiff + tDiff
	return data.FormatBytesU64(newSum), bytesPerSecond(diffSum, interval)
}

func totalSummary(oldState, newState uploadStats, interval time.Duration) []string {
	oldA := oldState.arbitrary.uploadedFileCount
	oldB := oldState.binary.uploadedFileCount
	oldT := oldState.tabular.uploadedFileCount
	newA := newState.arbitrary.uploadedFileCount
	newB := newState.binary.uploadedFileCount
	newT := newState.tabular.uploadedFileCount
	sumFileCount, sumDiffFileCountPerSec := totalCountDiff(oldA, oldB, oldT, newA, newB, newT, interval)
	file := fmt.Sprintf("total uploads: %d, rate: %s/sec", sumFileCount, sumDiffFileCountPerSec)

	oldA = oldState.arbitrary.uploadedBytes
	oldB = oldState.binary.uploadedBytes
	oldT = oldState.tabular.uploadedBytes
	newA = newState.arbitrary.uploadedBytes
	newB = newState.binary.uploadedBytes
	newT = newState.tabular.uploadedBytes
	sumBytesCount, sumDiffBytesPerSec := totalBytesDiff(oldA, oldB, oldT, newA, newB, newT, interval)
	bytes := fmt.Sprintf("total uploaded: %s, rate: %s/sec", sumBytesCount, sumDiffBytesPerSec)

	oldA = oldState.arbitrary.uploadFailedFileCount
	oldB = oldState.binary.uploadFailedFileCount
	oldT = oldState.tabular.uploadFailedFileCount
	newA = newState.arbitrary.uploadFailedFileCount
	newB = newState.binary.uploadFailedFileCount
	newT = newState.tabular.uploadFailedFileCount
	sumFailedCount, sumDiffFailedFileCountPerSec := totalCountDiff(oldA, oldB, oldT, newA, newB, newT, interval)
	failed := fmt.Sprintf("total failed uploads: %d, rate: %s/sec", sumFailedCount, sumDiffFailedFileCountPerSec)

	return []string{file, bytes, failed}
}

func summarizeStat(summary []string, label string, o, n stat, interval time.Duration) []string {
	var empty stat
	if o == empty && n == empty {
		return summary
	}
	successRate := perSecond(n.uploadedFileCount-o.uploadedFileCount, interval)
	successByteRate := bytesPerSecond(n.uploadedBytes-o.uploadedBytes, interval)
	failRate := perSecond(n.uploadFailedFileCount-o.uploadFailedFileCount, interval)
	summary = append(summary, fmt.Sprintf("%s file, (files uploaded): total: %d, rate: %f/sec",
		label, n.uploadedFileCount, successRate))
	summary = append(summary, fmt.Sprintf("%s file, (uploaded): total: %s, rate: %s/sec",
		label, data.FormatBytesU64(n.uploadedBytes), successByteRate))
	summary = append(summary, fmt.Sprintf("%s file, (failed file uploads): total: %d, rate: %f/sec",
		label, n.uploadFailedFileCount, failRate))
	return summary
}

func bytesPerSecond(v uint64, interval time.Duration) string {
	return data.FormatBytesU64(uint64(math.Floor(perSecond(v, interval))))
}

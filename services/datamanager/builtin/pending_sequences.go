package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/builtin/capture"
	datasync "go.viam.com/rdk/services/datamanager/builtin/sync"
)

// On-disk lifecycle (mirrors data capture's .prog → .capture):
//   <id>.progseq — open sequence; sync skips these
//   <id>.seq     — closed sequence ready for upload; sync picks these up
//
// On startup, orphaned .progseq files (left from a crashed previous process) are
// finalized to .seq with end_at = file mtime.
//
// The on-disk schema lives in the sync package (datasync.PendingSequenceFile) so
// writer (here) and reader (sync) share a single source of truth.

func sequencesPath(captureDir string) string {
	return filepath.Join(captureDir, datasync.SequencesDir)
}

func progSeqFilePath(captureDir, id string) string {
	return filepath.Join(sequencesPath(captureDir), id+data.InProgressSequenceFileExt)
}

func seqFilePath(captureDir, id string) string {
	return filepath.Join(sequencesPath(captureDir), id+data.CompletedSequenceFileExt)
}

// writeOpenSequence persists an opened sequence to disk as <id>.progseq.
// Atomic via .tmp + rename so sync's walker never sees a half-written file.
func writeOpenSequence(captureDir string, opened capture.OpenedSequence) error {
	pf := datasync.PendingSequenceFile{
		StartAt:      opened.StartAt,
		Resources:    toPendingResources(opened.Resources),
		SequenceTags: opened.Tags,
	}
	return writePendingSequenceFile(progSeqFilePath(captureDir, opened.ID), pf)
}

// writeClosedSequence persists a closed sequence to disk as <id>.seq and removes
// the corresponding <id>.progseq. The order — write .seq first, then remove .progseq —
// favors duplicate-on-crash (recoverable via dedup) over loss-on-crash.
func writeClosedSequence(captureDir string, closed capture.PendingSequence) error {
	pf := datasync.PendingSequenceFile{
		StartAt:      closed.StartAt,
		EndAt:        closed.EndAt,
		Resources:    toPendingResources(closed.Resources),
		SequenceTags: closed.SequenceTags,
	}
	if err := writePendingSequenceFile(seqFilePath(captureDir, closed.ID), pf); err != nil {
		return err
	}
	if err := os.Remove(progSeqFilePath(captureDir, closed.ID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove .progseq after writing .seq: %w", err)
	}
	return nil
}

// writePendingSequenceFile is the shared atomic-write primitive for both .progseq and .seq.
func writePendingSequenceFile(finalPath string, pf datasync.PendingSequenceFile) error {
	bytes, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pending sequence: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return fmt.Errorf("failed to create pending sequences dir: %w", err)
	}
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o600); err != nil {
		return fmt.Errorf("failed to write tmp file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename tmp file into place: %w", err)
	}
	return nil
}

func toPendingResources(rs []datamanager.ResourceMethod) []datasync.PendingSequenceResource {
	out := make([]datasync.PendingSequenceResource, len(rs))
	for i, r := range rs {
		out[i] = datasync.PendingSequenceResource{
			ResourceName: r.ResourceName,
			MethodName:   r.Method,
		}
	}
	return out
}

// persistOpenedSequences writes each opened sequence to disk as <id>.progseq.
func persistOpenedSequences(captureDir string, opened []capture.OpenedSequence, logger logging.Logger) {
	for _, o := range opened {
		if err := writeOpenSequence(captureDir, o); err != nil {
			logger.Errorw("failed to persist open sequence", "error", err, "id", o.ID, "start_at", o.StartAt)
			continue
		}
		logger.Debugw("persisted open sequence", "id", o.ID, "start_at", o.StartAt)
	}
}

// persistClosedSequences writes each closed sequence to disk as <id>.seq and removes the .progseq.
func persistClosedSequences(captureDir string, closed []capture.PendingSequence, logger logging.Logger) {
	for _, c := range closed {
		if err := writeClosedSequence(captureDir, c); err != nil {
			logger.Errorw("failed to persist closed sequence",
				"error", err, "id", c.ID, "start_at", c.StartAt, "end_at", c.EndAt)
			continue
		}
		logger.Infow("persisted closed sequence",
			"id", c.ID, "start_at", c.StartAt, "end_at", c.EndAt, "resource_count", len(c.Resources))
	}
}

// recoverOrphanedOpenSequences finalizes any .progseq files left from a previous process.
// For each orphan, end_at is set to the file's mtime (best approximation of when the
// previous process last wrote it). The orphan is then renamed to .seq for upload.
//
// Called once at startup, before the capture control poller begins, so no race with the
// normal close path.
func recoverOrphanedOpenSequences(captureDir string, logger logging.Logger) {
	dir := sequencesPath(captureDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnw("failed to scan pending sequences dir for orphans",
				"error", err, "dir", dir)
		}
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, data.InProgressSequenceFileExt) {
			continue
		}

		id := strings.TrimSuffix(name, data.InProgressSequenceFileExt)
		progPath := filepath.Join(dir, name)

		// If a .seq with the same id already exists, this .progseq is a duplicate from
		// a crash between writing .seq and removing .progseq. Just delete the orphan.
		if _, err := os.Stat(seqFilePath(captureDir, id)); err == nil {
			if rmErr := os.Remove(progPath); rmErr != nil {
				logger.Warnw("failed to remove duplicate orphan .progseq",
					"error", rmErr, "path", progPath)
			}
			continue
		}

		bytes, err := os.ReadFile(progPath) //nolint:gosec
		if err != nil {
			logger.Errorw("failed to read orphan .progseq; leaving on disk for inspection",
				"error", err, "path", progPath)
			continue
		}
		var pf datasync.PendingSequenceFile
		if err := json.Unmarshal(bytes, &pf); err != nil {
			logger.Errorw("failed to parse orphan .progseq; leaving on disk for inspection",
				"error", err, "path", progPath)
			continue
		}

		info, err := e.Info()
		if err != nil {
			logger.Errorw("failed to stat orphan .progseq", "error", err, "path", progPath)
			continue
		}
		pf.EndAt = info.ModTime()

		if err := writePendingSequenceFile(seqFilePath(captureDir, id), pf); err != nil {
			logger.Errorw("failed to finalize orphan .progseq to .seq",
				"error", err, "id", id)
			continue
		}
		if err := os.Remove(progPath); err != nil && !os.IsNotExist(err) {
			logger.Warnw("failed to remove orphan .progseq after finalization",
				"error", err, "path", progPath)
		}
		logger.Infow("recovered orphan open sequence",
			"id", id, "start_at", pf.StartAt, "end_at", pf.EndAt)
	}
}

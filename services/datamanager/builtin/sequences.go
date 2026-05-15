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

// Sequence files live in <captureDir>/sequences/

func sequencesPath(captureDir string) string {
	return filepath.Join(captureDir, datasync.SequencesDir)
}

func progSeqFilePath(captureDir, id string) string {
	return filepath.Join(sequencesPath(captureDir), id+data.InProgressSequenceFileExt)
}

func seqFilePath(captureDir, id string) string {
	return filepath.Join(sequencesPath(captureDir), id+data.CompletedSequenceFileExt)
}

// writeOpenSequence atomically writes opened to <id>.progseq.
func writeOpenSequence(captureDir string, opened capture.OpenedSequence) error {
	pf := datasync.SequenceFile{
		StartAt:      opened.StartAt,
		Resources:    toSequenceResources(opened.Resources),
		SequenceTags: opened.SequenceTags,
	}
	return writeSequenceFile(progSeqFilePath(captureDir, opened.ID), pf)
}

// writeClosedSequence atomically writes closed to <id>.seq and removes <id>.progseq.
// Order favors duplicate-on-crash (deduped on recovery) over loss-on-crash.
func writeClosedSequence(captureDir string, closed capture.Sequence) error {
	pf := datasync.SequenceFile{
		StartAt:      closed.StartAt,
		EndAt:        closed.EndAt,
		Resources:    toSequenceResources(closed.Resources),
		SequenceTags: closed.SequenceTags,
	}
	if err := writeSequenceFile(seqFilePath(captureDir, closed.ID), pf); err != nil {
		return err
	}
	if err := os.Remove(progSeqFilePath(captureDir, closed.ID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove .progseq after writing .seq: %w", err)
	}
	return nil
}

// writeSequenceFile writes pf to finalPath via .tmp + rename.
func writeSequenceFile(finalPath string, pf datasync.SequenceFile) error {
	bytes, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sequence: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return fmt.Errorf("failed to create sequences dir: %w", err)
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

func toSequenceResources(rs []datamanager.ResourceMethod) []datasync.SequenceResource {
	out := make([]datasync.SequenceResource, len(rs))
	for i, r := range rs {
		out[i] = datasync.SequenceResource{
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
func persistClosedSequences(captureDir string, closed []capture.Sequence, logger logging.Logger) {
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

// recoverOrphanedOpenSequences finalizes .progseq files left from a crashed previous process
// by setting end_at to the file's mtime and writing them out as .seq. Called once at startup.
func recoverOrphanedOpenSequences(captureDir string, logger logging.Logger) {
	dir := sequencesPath(captureDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnw("failed to scan sequences dir for orphans",
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

		// A matching .seq means we crashed between writing it and removing the .progseq; dedup.
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
		var pf datasync.SequenceFile
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

		if err := writeSequenceFile(seqFilePath(captureDir, id), pf); err != nil {
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

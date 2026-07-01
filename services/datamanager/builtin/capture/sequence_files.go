package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"braces.dev/errtrace"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/services/datamanager"
)

func sequencesPath(captureDir string) string {
	return filepath.Join(captureDir, data.SequencesDir)
}

func progSeqFilePath(captureDir, id string) string {
	return filepath.Join(sequencesPath(captureDir), id+data.InProgressSequenceFileExt)
}

func seqFilePath(captureDir, id string) string {
	return filepath.Join(sequencesPath(captureDir), id+data.CompletedSequenceFileExt)
}

// writeOpenSequence writes <id>.progseq for an opened sequence.
func writeOpenSequence(captureDir string, opened OpenSequence) error {
	sf := data.SequenceFile{
		StartTime:    opened.StartAt,
		Resources:    toSequenceResources(opened.Resources),
		SequenceTags: opened.SequenceTags,
	}
	return errtrace.Wrap(writeSequenceFile(progSeqFilePath(captureDir, opened.ID), sf))
}

// writeClosedSequence writes <id>.seq and removes the corresponding <id>.progseq.
func writeClosedSequence(captureDir string, closed ClosedSequence) error {
	sf := data.SequenceFile{
		StartTime:    closed.StartAt,
		EndTime:      closed.EndAt,
		Resources:    toSequenceResources(closed.Resources),
		SequenceTags: closed.SequenceTags,
	}
	if err := writeSequenceFile(seqFilePath(captureDir, closed.ID), sf); err != nil {
		return errtrace.Wrap(err)
	}
	if err := os.Remove(progSeqFilePath(captureDir, closed.ID)); err != nil && !os.IsNotExist(err) {
		return errtrace.Wrap(fmt.Errorf("failed to remove .progseq after writing .seq: %w", err))
	}
	return nil
}

// writeSequenceFile marshals sf to JSON and writes it to finalPath atomically (via .tmp +
// rename) so the sync walker never sees a partial file. Creates the parent dir if needed.
func writeSequenceFile(finalPath string, sf data.SequenceFile) error {
	bytes, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return errtrace.Wrap(fmt.Errorf("failed to marshal sequence: %w", err))
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return errtrace.Wrap(fmt.Errorf("failed to create sequences dir: %w", err))
	}
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o600); err != nil {
		return errtrace.Wrap(fmt.Errorf("failed to write sequence tmp file: %w", err))
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath) //nolint:errcheck
		return errtrace.Wrap(fmt.Errorf("failed to rename sequence tmp file: %w", err))
	}
	return nil
}

func toSequenceResources(rs []datamanager.ResourceMethod) []data.SequenceResource {
	out := make([]data.SequenceResource, len(rs))
	for i, r := range rs {
		out[i] = data.SequenceResource(r)
	}
	return out
}

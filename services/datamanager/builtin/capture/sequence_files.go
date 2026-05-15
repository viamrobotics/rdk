package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	pf := data.SequenceFile{
		StartAt:      opened.StartAt,
		Resources:    toSequenceResources(opened.Resources),
		SequenceTags: opened.SequenceTags,
	}
	return writeSequenceFile(progSeqFilePath(captureDir, opened.ID), pf)
}

// writeClosedSequence writes <id>.seq and removes the corresponding <id>.progseq.
// Order favors duplicate-on-crash (deduped on recovery) over loss-on-crash.
func writeClosedSequence(captureDir string, closed ClosedSequence) error {
	pf := data.SequenceFile{
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

// writeSequenceFile marshals pf to JSON and writes it to finalPath, creating the parent
// directory if needed. Shared by writeOpenSequence and writeClosedSequence.
func writeSequenceFile(finalPath string, pf data.SequenceFile) error {
	bytes, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sequence: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return fmt.Errorf("failed to create sequences dir: %w", err)
	}
	if err := os.WriteFile(finalPath, bytes, 0o600); err != nil {
		return fmt.Errorf("failed to write sequence file: %w", err)
	}
	return nil
}

func toSequenceResources(rs []datamanager.ResourceMethod) []data.SequenceResource {
	out := make([]data.SequenceResource, len(rs))
	for i, r := range rs {
		out[i] = data.SequenceResource{
			ResourceName: r.ResourceName,
			MethodName:   r.Method,
		}
	}
	return out
}

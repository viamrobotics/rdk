package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
)

// syncSequence uploads a .seq file via CreateSequence, using exponentialRetry for transient errors.
// On terminal error the file is moved to failed/; on success it is removed.
func (s *Sync) syncSequence(filePath string) {
	logger := s.logger
	bytes, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		logger.Errorw("failed to read sequence file", "path", filePath, "error", err)
		return
	}

	var sf data.SequenceFile
	if err := json.Unmarshal(bytes, &sf); err != nil {
		logger.Errorw("failed to parse sequence file; moving to failed",
			"path", filePath, "error", err)
		moveSequenceToFailed(filePath, errors.Wrap(err, "unmarshal"), logger)
		return
	}

	retry := newExponentialRetry(s.configCtx, s.clock, s.logger, filePath, func(ctx context.Context) (uint64, error) {
		if s.cloudConn.dataClient == nil {
			return 0, errors.New("cloud connection not ready")
		}
		req := sequenceRequest(&sf, s.cloudConn.partID)
		_, err := s.cloudConn.dataClient.CreateSequence(ctx, req)
		return 0, err
	})

	if _, err := retry.run(); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		logger.Errorw("CreateSequence hit terminal error; moving to failed",
			"path", filePath, "error", err)
		moveSequenceToFailed(filePath, err, logger)
		return
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warnw("failed to remove uploaded sequence file", "path", filePath, "error", err)
	}
}

func sequenceRequest(sf *data.SequenceFile, partID string) *datapb.CreateSequenceRequest {
	resources := make([]*datapb.SequenceResourceFilter, len(sf.Resources))
	for i, r := range sf.Resources {
		resources[i] = &datapb.SequenceResourceFilter{
			ResourceName: r.ResourceName,
			MethodName:   r.MethodName,
		}
	}
	return &datapb.CreateSequenceRequest{
		PartId:       partID,
		Resources:    resources,
		SequenceTags: sf.SequenceTags,
		StartTime:    timestamppb.New(sf.StartTime),
		EndTime:      timestamppb.New(sf.EndTime),
	}
}

// moveSequenceToFailed moves path to <captureDir>/failed/sequences/ for operator inspection.
// path is expected to be <captureDir>/sequences/<id>.{seq,progseq}.
func moveSequenceToFailed(path string, cause error, logger logging.Logger) {
	captureDir := filepath.Dir(filepath.Dir(path))
	if err := moveFailedData(path, captureDir, cause, logger); err != nil {
		logger.Errorw("failed to move sequence to failed/", "path", path, "error", err)
	}
}

// handleOrphanedOpenSequences cleans up .progseq files left from a crashed previous process.
// Without a clean end_at we can't upload them, so we move them to <captureDir>/failed/sequences/.
func handleOrphanedOpenSequences(captureDir string, logger logging.Logger) {
	dir := filepath.Join(captureDir, data.SequencesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warnw("failed to scan sequences dir for orphans", "error", err, "dir", dir)
		}
		return
	}
	failedDir := filepath.Join(captureDir, FailedDir, data.SequencesDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join(dir, name)
		// Stray .tmp files are partial atomic writes from a prior crash. Always junk; remove.
		if strings.HasSuffix(name, ".tmp") {
			if err := os.Remove(path); err != nil {
				logger.Warnw("failed to remove orphan sequence .tmp", "error", err, "path", path)
			}
			continue
		}
		if !strings.HasSuffix(name, data.InProgressSequenceFileExt) {
			continue
		}
		id := strings.TrimSuffix(name, data.InProgressSequenceFileExt)
		seqPath := filepath.Join(dir, id+data.CompletedSequenceFileExt)

		// A matching .seq means we crashed between writing it and removing the .progseq; dedup.
		if _, err := os.Stat(seqPath); err == nil {
			if rmErr := os.Remove(path); rmErr != nil {
				logger.Warnw("failed to remove duplicate orphan .progseq",
					"error", rmErr, "path", path)
			}
			continue
		}

		if err := os.MkdirAll(failedDir, 0o700); err != nil {
			logger.Errorw("failed to create failed/ for orphan", "error", err, "dir", failedDir)
			continue
		}
		dst := filepath.Join(failedDir, name)
		if err := os.Rename(path, dst); err != nil {
			logger.Errorw("failed to move orphan .progseq to failed/", "error", err, "path", path)
			continue
		}
		logger.Warnw("moved orphan open sequence to failed/", "path", dst)
	}
}

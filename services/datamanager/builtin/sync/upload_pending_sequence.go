package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
)

// PendingSequenceFile is the on-disk representation of a closed sequence recording.
type PendingSequenceFile struct {
	StartAt      time.Time                 `json:"start_at"`
	EndAt        time.Time                 `json:"end_at"`
	Resources    []PendingSequenceResource `json:"resources"`
	SequenceTags []string                  `json:"sequence_tags,omitempty"`
}

// PendingSequenceResource uses the cloud-API field names so building the proto request
// is a direct copy with no translation.
type PendingSequenceResource struct {
	ResourceName string `json:"resource_name"`
	MethodName   string `json:"method_name"`
}

// syncPendingSequence reads a pending sequence JSON file and calls CreateSequence.
// Uses the existing exponentialRetry helper to handle transient errors with backoff.
// On terminal failure the file is moved to <pending_sequences>/<FailedDir>/. On success it is deleted.
func (s *Sync) syncPendingSequence(_ Config, filePath string) {
	logger := s.logger
	bytes, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		logger.Errorw("failed to read pending sequence file", "path", filePath, "error", err)
		return
	}

	var pf PendingSequenceFile
	if err := json.Unmarshal(bytes, &pf); err != nil {
		// Unparseable file is permanent — move to failed/ and continue.
		logger.Errorw("failed to parse pending sequence file; moving to failed",
			"path", filePath, "error", err)
		movePendingSequenceToFailed(filePath, errors.Wrap(err, "unmarshal"), logger)
		return
	}
	if err := validatePendingSequenceFile(&pf); err != nil {
		logger.Errorw("invalid pending sequence file; moving to failed",
			"path", filePath, "error", err)
		movePendingSequenceToFailed(filePath, err, logger)
		return
	}

	retry := newExponentialRetry(s.configCtx, s.clock, s.logger, filePath, func(ctx context.Context) (uint64, error) {
		if s.cloudConn.appConn == nil {
			return 0, errors.New("cloud connection not ready")
		}
		req := pendingSequenceRequest(&pf, s.cloudConn.partID)
		client := datapb.NewDataServiceClient(s.cloudConn.appConn)
		if _, err := client.CreateSequence(ctx, req); err != nil {
			return 0, errors.Wrap(err, "CreateSequence failed")
		}
		return 0, nil
	})

	if _, err := retry.run(); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		logger.Errorw("CreateSequence hit terminal error; moving to failed",
			"path", filePath, "error", err)
		movePendingSequenceToFailed(filePath, err, logger)
		return
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warnw("failed to remove uploaded pending sequence file",
			"path", filePath, "error", err)
	}
}

func validatePendingSequenceFile(pf *PendingSequenceFile) error {
	if len(pf.Resources) == 0 {
		return errors.New("at least one resource is required")
	}
	return nil
}

func pendingSequenceRequest(pf *PendingSequenceFile, partID string) *datapb.CreateSequenceRequest {
	resources := make([]*datapb.SequenceResourceFilter, len(pf.Resources))
	for i, r := range pf.Resources {
		resources[i] = &datapb.SequenceResourceFilter{
			ResourceName: r.ResourceName,
			MethodName:   r.MethodName,
		}
	}
	return &datapb.CreateSequenceRequest{
		PartId:       partID,
		Resources:    resources,
		SequenceTags: pf.SequenceTags,
		StartTime:    timestamppb.New(pf.StartAt),
		EndTime:      timestamppb.New(pf.EndAt),
	}
}

// movePendingSequenceToFailed moves the file to <pending_sequences>/<FailedDir>/ so the
// operator can inspect it. Reuses sync's existing moveFailedData helper for consistency
// with how data capture files handle permanent upload failures.
func movePendingSequenceToFailed(path string, cause error, logger logging.Logger) {
	// pending_sequences/<file> -> pending_sequences/failed/<file>
	// moveFailedData takes parentDir as the dir that will contain the failed/ subdir.
	parentDir := filepath.Dir(path)
	if err := moveFailedData(path, parentDir, cause, logger); err != nil {
		logger.Errorw("failed to move pending sequence to failed/", "path", path, "error", err)
	}
}

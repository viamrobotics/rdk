package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
)

// SequenceFile is the on-disk representation of a sequence.
type SequenceFile struct {
	StartAt      time.Time          `json:"start_at"`
	EndAt        time.Time          `json:"end_at"`
	Resources    []SequenceResource `json:"resources"`
	SequenceTags []string           `json:"sequence_tags,omitempty"`
}

// SequenceResource is the on-disk form of a resource/method pair in a sequence.
type SequenceResource struct {
	ResourceName string `json:"resource_name"`
	MethodName   string `json:"method_name"`
}

// syncSequence uploads a .seq file via CreateSequence. The walker provides the retry cadence —
// on transient errors we leave the file on disk for the next walker pass to pick up.
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

	var pf SequenceFile
	if err := json.Unmarshal(bytes, &pf); err != nil {
		logger.Errorw("failed to parse sequence file; moving to failed",
			"path", filePath, "error", err)
		moveSequenceToFailed(filePath, errors.Wrap(err, "unmarshal"), logger)
		return
	}
	if err := validateSequenceFile(&pf); err != nil {
		logger.Errorw("invalid sequence file; moving to failed",
			"path", filePath, "error", err)
		moveSequenceToFailed(filePath, err, logger)
		return
	}

	if s.cloudConn.dataClient == nil {
		// cloud connection not ready; walker will retry on the next pass.
		return
	}
	req := sequenceRequest(&pf, s.cloudConn.partID)
	if _, err := s.cloudConn.dataClient.CreateSequence(s.configCtx, req); err != nil {
		if isPermanentSequenceError(err) {
			logger.Errorw("CreateSequence rejected permanently; moving to failed",
				"path", filePath, "error", err)
			moveSequenceToFailed(filePath, err, logger)
			return
		}
		logger.Warnw("CreateSequence failed; will retry on next walker pass",
			"path", filePath, "error", err)
		return
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Warnw("failed to remove uploaded sequence file", "path", filePath, "error", err)
	}
}

func validateSequenceFile(pf *SequenceFile) error {
	if len(pf.Resources) == 0 {
		return errors.New("at least one resource is required")
	}
	return nil
}

func sequenceRequest(pf *SequenceFile, partID string) *datapb.CreateSequenceRequest {
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

// moveSequenceToFailed moves path to sequences/failed/ for operator inspection.
func moveSequenceToFailed(path string, cause error, logger logging.Logger) {
	if err := moveFailedData(path, filepath.Dir(path), cause, logger); err != nil {
		logger.Errorw("failed to move sequence to failed/", "path", path, "error", err)
	}
}

// isPermanentSequenceError returns true for gRPC errors that retry won't fix.
func isPermanentSequenceError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	c := st.Code()
	return c == codes.InvalidArgument ||
		c == codes.PermissionDenied ||
		c == codes.NotFound ||
		c == codes.Unauthenticated ||
		c == codes.FailedPrecondition
}

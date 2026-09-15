package sync

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/robot"
)

// UploadDataFromPath uploads a file or all files in a directory at path to the cloud.
// For directories, every file is attempted and per-file errors are counted; the call
// returns aggregate counts of files and bytes uploaded/failed.
// TODO (DATA-4528): Don't ignore the extra field in the UploadDataFromPath request.
func (s *Sync) UploadDataFromPath(ctx context.Context, path string, uploadMetadata *v1.UploadMetadata, extra map[string]interface{}) (
	robot.UploadDataFromPathResult, error,
) {
	select {
	case <-s.cloudConn.ready:
	default:
		return robot.UploadDataFromPathResult{}, errors.New("not connected to the cloud")
	}

	s.configMu.Lock()
	configCtx := s.configCtx
	s.configMu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(configCtx, cancel)
	defer stop()

	info, err := os.Stat(path)
	if err != nil {
		return robot.UploadDataFromPathResult{}, errors.Wrapf(err, "failed to stat file path %s", path)
	}

	var result robot.UploadDataFromPathResult

	tags := uploadMetadata.GetTags()
	datasetIDs := uploadMetadata.GetDatasetIds()

	uploadOne := func(filePath string) {
		// sequence files and data capture files are managed by data capture, so skip and log
		if isSequenceFile(filePath) || isOpenSequenceFile(filePath) ||
			isCompletedCaptureFile(filePath) || filepath.Ext(filePath) == data.InProgressCaptureFileExt {
			s.logger.Warnw("skipping file managed by data capture:", "path", filePath)
			return
		}

		fi, statErr := os.Stat(filePath)
		if statErr != nil {
			s.logger.Errorw("failed to stat file for upload", "path", filePath, "error", statErr)
			result.FilesFailed++
			return
		}

		//nolint:gosec
		f, openErr := os.Open(filePath)
		if openErr != nil {
			s.logger.Errorw("failed to open file for upload", "path", filePath, "error", openErr)
			result.FilesFailed++
			return
		}

		result.BytesTotal += uint64(fi.Size())

		// This path bypasses the sync-worker fileTracker, so the progress logger's
		// lifetime is managed locally: close always cleans up the goroutine, while
		// complete/failure log the summary and record the FTDC stats.
		progress := newUploadProgressLogger(s.logger, s.clock, filePath, fi.Size(), &s.uploadStats.frompath)
		defer progress.close()

		progress.startAttempt()
		uploadedBytes, id, uploadErr := uploadArbitraryFile(ctx, f, s.cloudConn,
			tags, datasetIDs, 0, s.clock, s.logger, progress)
		if uploadErr != nil {
			progress.attemptFailed()
		}
		progress.onResult(uploadedBytes, uploadErr)
		if closeErr := f.Close(); closeErr != nil {
			s.logger.Warnw("failed to close file after upload data from path request", "path", filePath, "error", closeErr)
		}
		if uploadErr != nil {
			result.FilesFailed++
			// Unlike the scheduled sync paths, this calls uploadArbitraryFile directly (no
			// retry wrapper), so wrap the error to identify the path for logUploadOutcome.
			logUploadOutcome(s.logger, filePath, errors.Wrapf(uploadErr, "error uploading file from path %s", filePath))
			return
		}

		result.FilesUploaded++
		result.BytesUploaded += uploadedBytes
		if id != "" {
			result.IDs = append(result.IDs, id)
		}
	}

	if info.IsDir() {
		err = filepath.Walk(path, func(filePath string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				s.logger.Errorw("error accessing path during walk, skipping", "path", filePath, "error", walkErr)
				return nil
			}
			if fi.IsDir() {
				return nil
			}
			uploadOne(filePath)
			return ctx.Err()
		})
	} else {
		uploadOne(path)
	}

	if err == nil {
		err = ctx.Err()
	}

	return result, err
}

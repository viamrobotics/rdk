package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	"go.viam.com/utils"
)

// sequenceBinaryDir is the directory under the user's destination where
// binary blobs land. It matches the `path` column the server writes into
// binary_data.parquet so each row's path resolves locally.
const sequenceBinaryDir = "binary_data"

// downloadSequenceDataset starts an async Parquet export of a sequence
// dataset, polls until it completes, and streams the resulting zip to
// dst/<datasetID>.zip. dst is created if it does not exist.
//
// When downloadBinaryData is true, also downloads every binary blob
// referenced by binary_data.parquet into dst/binary_data/<id><ext>.
// parallel and timeout govern the blob downloads (mirroring the binary
// dataset flow).
func (c *viamClient) downloadSequenceDataset(
	ctx context.Context, datasetID, dst string,
	pollInterval, maxWait time.Duration,
	downloadBinaryData bool, parallel, timeout uint,
) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return errors.Wrapf(err, "could not create destination directory %s", dst)
	}

	printf(c.c.Root().Writer, "Starting export for dataset %s", datasetID)
	startResp, err := c.datasetClient.StartSequenceDatasetExport(
		ctx, &datasetpb.StartSequenceDatasetExportRequest{DatasetId: datasetID},
	)
	if err != nil {
		return errors.Wrap(err, "failed to start sequence dataset export")
	}
	jobID := startResp.GetJobId()
	printf(c.c.Root().Writer, "Export job %s queued; polling every %s (max %s)", jobID, pollInterval, maxWait)

	getResp, err := c.pollUntilTerminal(ctx, jobID, pollInterval, maxWait)
	if err != nil {
		return err
	}

	dstPath := filepath.Join(dst, datasetID+".zip")
	if err := downloadSignedURL(ctx, getResp.GetDownloadUrl(), dstPath); err != nil {
		return err
	}
	printf(c.c.Root().Writer, "Wrote %s", dstPath)

	if !downloadBinaryData {
		return nil
	}
	return c.downloadSequenceBinaryBlobs(ctx, datasetID, dst, parallel, timeout)
}

type sequenceBlobJob struct {
	binaryDataID string
	fileExt      string
}

// downloadSequenceBinaryBlobs enumerates every binary record referenced by
// the dataset's sequences (via SequencesByDatasetID + GetSequenceBinaryData)
// and downloads each blob into dst/binary_data/<binary_data_id><file_ext>,
// matching the relative `path` column the server wrote into binary_data.parquet.
//
// Fail-fast on first error with multierr aggregation — mirrors the binary
// dataset flow's performActionOnBinaryDataFromFilter.
func (c *viamClient) downloadSequenceBinaryBlobs(
	ctx context.Context, datasetID, dst string, parallel, timeout uint,
) error {
	binaryDir := filepath.Join(dst, sequenceBinaryDir)
	if err := os.MkdirAll(binaryDir, 0o700); err != nil {
		return errors.Wrapf(err, "could not create %s", binaryDir)
	}
	if parallel == 0 {
		parallel = 100
	}

	printf(c.c.Root().Writer, "Downloading binary blobs to %s", binaryDir)

	work := make(chan sequenceBlobJob, parallel)
	errs := make(chan error, 1+parallel)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var producerWG sync.WaitGroup
	producerWG.Add(1)
	go func() {
		defer producerWG.Done()
		defer close(work)
		if err := c.streamSequenceBinaryJobs(ctx, datasetID, work); err != nil {
			errs <- err
			cancel()
		}
	}()

	var workerWG sync.WaitGroup
	var done atomic.Int32
	for i := uint(0); i < parallel; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for job := range work {
				if err := c.fetchAndWriteSequenceBlob(ctx, binaryDir, job, timeout); err != nil {
					errs <- err
					cancel()
					return
				}
				if n := done.Add(1); n%100 == 0 {
					printf(c.c.Root().Writer, "Downloaded %d binary blobs", n)
				}
			}
		}()
	}

	producerWG.Wait()
	workerWG.Wait()
	close(errs)

	var allErrs error
	for e := range errs {
		allErrs = multierr.Append(allErrs, e)
	}
	if allErrs == nil {
		printf(c.c.Root().Writer, "Done — wrote %d binary blobs to %s", done.Load(), binaryDir)
	}
	return allErrs
}

// streamSequenceBinaryJobs pages through every sequence in the dataset and,
// for each, pages through its binary data records, pushing one job per blob
// into work. Returns the first error it encounters.
func (c *viamClient) streamSequenceBinaryJobs(
	ctx context.Context, datasetID string, work chan<- sequenceBlobJob,
) error {
	sequenceIDs, err := c.allSequenceIDsForDataset(ctx, datasetID)
	if err != nil {
		return err
	}
	for _, seqID := range sequenceIDs {
		pageToken := ""
		for {
			resp, err := c.dataClient.GetSequenceBinaryData(ctx, &datapb.GetSequenceBinaryDataRequest{
				SequenceId: seqID,
				PageToken:  pageToken,
			})
			if err != nil {
				return errors.Wrapf(err, "failed to list binary data for sequence %s", seqID)
			}
			for _, bd := range resp.GetData() {
				job := sequenceBlobJob{
					binaryDataID: bd.GetMetadata().GetBinaryDataId(),
					fileExt:      bd.GetMetadata().GetFileExt(),
				}
				select {
				case work <- job:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			pageToken = resp.GetNextPageToken()
			if pageToken == "" {
				break
			}
		}
	}
	return nil
}

func (c *viamClient) allSequenceIDsForDataset(
	ctx context.Context, datasetID string,
) ([]string, error) {
	var ids []string
	pageToken := ""
	for {
		resp, err := c.dataClient.SequencesByDatasetID(ctx, &datapb.SequencesByDatasetIDRequest{
			DatasetId: datasetID,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to list sequences for dataset")
		}
		for _, seq := range resp.GetSequences() {
			ids = append(ids, seq.GetId())
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			return ids, nil
		}
	}
}

// fetchAndWriteSequenceBlob downloads one binary by ID and writes its bytes
// to binaryDir/<id><ext>. Already-present files with non-zero size are
// skipped so re-runs after a partial download don't redo work.
func (c *viamClient) fetchAndWriteSequenceBlob(
	ctx context.Context, binaryDir string, job sequenceBlobJob, timeout uint,
) error {
	dstPath := filepath.Join(binaryDir, job.binaryDataID+job.fileExt)
	if info, err := os.Stat(dstPath); err == nil && info.Size() > 0 {
		return nil
	}

	callCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	resp, err := c.dataClient.BinaryDataByIDs(callCtx, &datapb.BinaryDataByIDsRequest{
		BinaryDataIds: []string{job.binaryDataID},
		IncludeBinary: true,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to fetch binary data %s", job.binaryDataID)
	}
	if len(resp.GetData()) == 0 {
		return fmt.Errorf("binary data %s not found", job.binaryDataID)
	}
	// binaryDataID can contain slashes (org/part/id), so dstPath may be nested
	// below binaryDir; create the intermediate directories before writing.
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
		return errors.Wrapf(err, "failed to create directory for %s", dstPath)
	}
	if err := os.WriteFile(dstPath, resp.GetData()[0].GetBinary(), 0o600); err != nil {
		return errors.Wrapf(err, "failed to write %s", dstPath)
	}
	return nil
}

// downloadSignedURL streams the GET response for signedURL into dst, creating
// or truncating dst. Returns an error if the HTTP status is not 2xx.
func downloadSignedURL(ctx context.Context, signedURL, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL, nil)
	if err != nil {
		return errors.Wrap(err, "building download request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "downloading export zip")
	}
	//nolint:errcheck
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) //nolint:errcheck
		return fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(body))
	}

	out, err := os.Create(dst) //nolint:gosec
	if err != nil {
		return errors.Wrapf(err, "could not create %s", dst)
	}
	//nolint:errcheck
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		utils.UncheckedError(os.Remove(dst))
		return errors.Wrap(err, "writing export zip to disk")
	}
	return nil
}

// pollUntilTerminal calls GetSequenceDatasetExport every pollInterval until
// the job leaves RUNNING. Returns the terminal response on COMPLETED;
// returns an error on FAILED, ctx cancellation, or after maxWait elapses.
func (c *viamClient) pollUntilTerminal(
	ctx context.Context, jobID string, pollInterval, maxWait time.Duration,
) (*datasetpb.GetSequenceDatasetExportResponse, error) {
	deadline := time.Now().Add(maxWait)
	for {
		resp, err := c.datasetClient.GetSequenceDatasetExport(ctx, &datasetpb.GetSequenceDatasetExportRequest{JobId: jobID})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fetch export job status")
		}
		switch resp.GetStatus() {
		case datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_COMPLETED:
			return resp, nil
		case datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_FAILED:
			return nil, fmt.Errorf("export job %s failed: %s", jobID, resp.GetErrorMessage())
		case datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_RUNNING,
			datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_UNSPECIFIED:
			// keep polling
		default:
			return nil, fmt.Errorf("export job %s returned unknown status: %s", jobID, resp.GetStatus())
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("export job %s timed out after %s; still RUNNING", jobID, maxWait)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

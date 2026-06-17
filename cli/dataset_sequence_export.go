package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	datasetpb "go.viam.com/api/app/dataset/v1"
)

// downloadSequenceDataset starts an async Parquet export of a sequence
// dataset, polls until it completes, and streams the resulting zip to
// dst/<datasetID>.zip. dst must already exist on disk.
func (c *viamClient) downloadSequenceDataset(
	ctx context.Context, datasetID, dst string, pollInterval, maxWait time.Duration,
) error {
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
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return errors.Wrapf(err, "could not create destination directory %s", dst)
	}
	if err := downloadSignedURL(ctx, getResp.GetDownloadUrl(), dstPath); err != nil {
		return err
	}
	printf(c.c.Root().Writer, "Wrote %s", dstPath)
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(body))
	}

	out, err := os.Create(dst)
	if err != nil {
		return errors.Wrapf(err, "could not create %s", dst)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = os.Remove(dst)
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
		case datasetpb.SequenceDatasetExportStatus_SEQUENCE_DATASET_EXPORT_STATUS_RUNNING:
			// fallthrough to wait
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

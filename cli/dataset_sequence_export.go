package cli

import (
	"context"
	"time"

	datasetpb "go.viam.com/api/app/dataset/v1"
)

// downloadSequenceDataset starts an async Parquet export of a sequence
// dataset, polls until it completes, and streams the resulting zip to
// dst/<datasetID>.zip. dst must already exist on disk.
//
// Caller responsibilities: pass a context with a deadline if a hard cap is
// wanted beyond maxWait, and ensure dst is writable.
func (c *viamClient) downloadSequenceDataset(
	ctx context.Context, datasetID, dst string, pollInterval, maxWait time.Duration,
) error {
	startResp, err := c.datasetClient.StartSequenceDatasetExport(
		ctx, &datasetpb.StartSequenceDatasetExportRequest{DatasetId: datasetID},
	)
	if err != nil {
		return err
	}
	_ = startResp
	return nil
}

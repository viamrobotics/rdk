package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	"go.viam.com/utils"
)

const (
	datasetFlagName          = "name"
	datasetFlagDatasetID     = "dataset-id"
	datasetFlagDatasetIDs    = "dataset-ids"
	dataFlagLocationID       = "location-id"
	dataFlagBinaryDataIDs    = "binary-data-ids"
	datasetFlagOnlyJSONLines = "only-jsonl"
)

type datasetCreateArgs struct {
	OrgID string
	Name  string
}

// DatasetCreateAction is the corresponding action for 'dataset create'.
func DatasetCreateAction(c *cli.Context, args datasetCreateArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.createDataset(args.OrgID, args.Name); err != nil {
		return err
	}
	return nil
}

// createDataset creates a dataset with the a dataset ID.
func (c *viamClient) createDataset(orgID, datasetName string) error {
	resp, err := c.datasetClient.CreateDataset(context.Background(),
		&datasetpb.CreateDatasetRequest{OrganizationId: orgID, Name: datasetName})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	printf(c.c.App.Writer, "Created dataset %s with dataset ID: %s", datasetName, resp.GetId())
	return nil
}

type datasetRenameArgs struct {
	DatasetID string
	Name      string
}

// DatasetRenameAction is the corresponding action for 'dataset rename'.
func DatasetRenameAction(c *cli.Context, args datasetRenameArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.renameDataset(args.DatasetID, args.Name); err != nil {
		return err
	}
	return nil
}

// renameDataset renames an existing datasetID with the newDatasetName.
func (c *viamClient) renameDataset(datasetID, newDatasetName string) error {
	_, err := c.datasetClient.RenameDataset(context.Background(),
		&datasetpb.RenameDatasetRequest{Id: datasetID, Name: newDatasetName})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	printf(c.c.App.Writer, "Dataset with ID %s renamed to %s", datasetID, newDatasetName)
	return nil
}

type datasetMergeArgs struct {
	OrgID      string
	Name       string
	DatasetIDs []string
}

// DatasetMergeAction is the corresponding action for 'dataset merge'.
func DatasetMergeAction(c *cli.Context, args datasetMergeArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	return client.mergeDatasets(args.OrgID, args.Name, args.DatasetIDs)
}

// mergeDatasets merges multiple datasets into a new dataset with the specified name.
func (c *viamClient) mergeDatasets(orgID, newDatasetName string, datasetIDs []string) error {
	// Use the dataset service client to call MergeDatasets
	// Note: This will fail until the MergeDatasetsRequest/Response types are implemented in the backend
	resp, err := c.datasetClient.MergeDatasets(context.Background(), &datasetpb.MergeDatasetsRequest{
		OrganizationId: orgID,
		Name:           newDatasetName,
		DatasetIds:     datasetIDs,
	})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	printf(c.c.App.Writer, "Successfully merged %d datasets into new dataset '%s' with ID: %s",
		len(datasetIDs), newDatasetName, resp.GetDatasetId())
	return nil
}

type datasetListArgs struct {
	DatasetIDs []string
	OrgID      string
}

// DatasetListAction is the corresponding action for 'dataset list'.
func DatasetListAction(c *cli.Context, args datasetListArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	datasetIDs := args.DatasetIDs
	orgID := args.OrgID

	if orgID != "" && datasetIDs != nil {
		return errors.New("must specify either dataset IDs or organization ID, got both")
	}
	if datasetIDs != nil {
		if err := client.listDatasetByIDs(datasetIDs); err != nil {
			return err
		}
	} else {
		if err := client.listDatasetByOrg(orgID); err != nil {
			return err
		}
	}

	return nil
}

// listDatasetByIDs list all datasets by ID.
func (c *viamClient) listDatasetByIDs(datasetIDs []string) error {
	resp, err := c.datasetClient.ListDatasetsByIDs(context.Background(),
		&datasetpb.ListDatasetsByIDsRequest{Ids: datasetIDs})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	for _, dataset := range resp.GetDatasets() {
		printf(c.c.App.Writer, "\t%s (ID: %s, Organization ID: %s)", dataset.Name, dataset.Id, dataset.OrganizationId)
	}
	return nil
}

// listDatasetByOrg list all datasets for the specified org ID.
func (c *viamClient) listDatasetByOrg(orgID string) error {
	resp, err := c.datasetClient.ListDatasetsByOrganizationID(context.Background(),
		&datasetpb.ListDatasetsByOrganizationIDRequest{OrganizationId: orgID})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	for _, dataset := range resp.GetDatasets() {
		printf(c.c.App.Writer, "\t%s (ID: %s, Organization ID: %s)", dataset.Name, dataset.Id, dataset.OrganizationId)
	}
	return nil
}

type datasetDeleteArgs struct {
	DatasetID string
}

// DatasetDeleteAction is the corresponding action for 'dataset delete'.
func DatasetDeleteAction(c *cli.Context, args datasetDeleteArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.deleteDataset(args.DatasetID); err != nil {
		return err
	}
	return nil
}

// deleteDataset deletes a dataset with the specified ID.
func (c *viamClient) deleteDataset(datasetID string) error {
	_, err := c.datasetClient.DeleteDataset(context.Background(),
		&datasetpb.DeleteDatasetRequest{Id: datasetID})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	printf(c.c.App.Writer, "Dataset with ID %s deleted", datasetID)
	return nil
}

type datasetDownloadArgs struct {
	Destination string
	DatasetID   string
	OnlyJSONl   bool
	Parallel    uint
	Timeout     uint
}

// DatasetDownloadAction is the corresponding action for 'dataset export'.
func DatasetDownloadAction(c *cli.Context, args datasetDownloadArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.downloadDataset(args.Destination, args.DatasetID,
		args.OnlyJSONl, args.Parallel, args.Timeout); err != nil {
		return err
	}
	return nil
}

// downloadDataset downloads a dataset with the specified ID.
func (c *viamClient) downloadDataset(dst, datasetID string, onlyJSONLines bool, parallelDownloads, timeout uint) error {
	var datasetFile *os.File
	var err error
	datasetPath := filepath.Join(dst, "dataset.jsonl")
	if err := os.MkdirAll(filepath.Dir(datasetPath), 0o700); err != nil {
		return errors.Wrapf(err, "could not create dataset directory %s", filepath.Dir(datasetPath))
	}
	//nolint:gosec
	datasetFile, err = os.Create(datasetPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := datasetFile.Close(); err != nil {
			Errorf(c.c.App.ErrWriter, "failed to close dataset file %q", datasetFile.Name())
		}
	}()

	resp, err := c.datasetClient.ListDatasetsByIDs(context.Background(),
		&datasetpb.ListDatasetsByIDsRequest{Ids: []string{datasetID}})
	if err != nil {
		return errors.Wrapf(err, "error getting dataset ID")
	}
	if len(resp.GetDatasets()) == 0 {
		return fmt.Errorf("%s does not match any dataset IDs", datasetID)
	}

	return c.performActionOnBinaryDataFromFilter(
		func(id string) error {
			var downloadErr error
			var datasetFilePath string
			if !onlyJSONLines {
				downloadErr = c.downloadBinary(dst, timeout, id)
				datasetFilePath = filepath.Join(dst, dataDir)
			}
			datasetErr := binaryIDToJSONLine(c, datasetFilePath, datasetFile, id, timeout)

			return multierr.Combine(downloadErr, datasetErr)
		},
		&datapb.Filter{
			DatasetId: datasetID,
		}, parallelDownloads,
		func(i int32) {
			printf(c.c.App.Writer, "Downloaded %d files", i)
		},
	)
}

// Annotation holds the label associated with the image.
type Annotation struct {
	AnnotationLabel string `json:"annotation_label"`
}

// ImageMetadata defines the format of the data in jsonlines for custom training.
type ImageMetadata struct {
	ImagePath                 string           `json:"image_path"`
	ClassificationAnnotations []Annotation     `json:"classification_annotations"`
	BBoxAnnotations           []BBoxAnnotation `json:"bounding_box_annotations"`
	Timestamp                 string           `json:"timestamp"`
	BinaryDataID              string           `json:"binary_data_id,omitempty"`
	OrganizationID            string           `json:"organization_id,omitempty"`
	RobotID                   string           `json:"robot_id,omitempty"`
	LocationID                string           `json:"location_id,omitempty"`
	PartID                    string           `json:"part_id,omitempty"`
	ComponentName             string           `json:"component_name,omitempty"`
}

// BBoxAnnotation holds the information associated with each bounding box.
type BBoxAnnotation struct {
	AnnotationLabel string  `json:"annotation_label"`
	XMinNormalized  float64 `json:"x_min_normalized"`
	XMaxNormalized  float64 `json:"x_max_normalized"`
	YMinNormalized  float64 `json:"y_min_normalized"`
	YMaxNormalized  float64 `json:"y_max_normalized"`
}

// BinaryMetadataToJSONLRequest represents the request body structure
type BinaryMetadataToJSONLRequest struct {
	BinaryMetadata []*datapb.BinaryMetadata `json:"binary_metadata"`
	Path           string                   `json:"path"`
}

func binaryIDToJSONLine(c *viamClient, path string, file *os.File, id string, timeout uint) error {
	args, err := getGlobalArgs(c.c)
	if err != nil {
		return err
	}
	debugf(c.c.App.Writer, args.Debug, "Attempting to get binary metadata for binary data ID: %s", id)
	var resp *datapb.BinaryDataByIDsResponse
	for count := 0; count < maxRetryCount; count++ {
		resp, err = c.dataClient.BinaryDataByIDs(c.c.Context, &datapb.BinaryDataByIDsRequest{
			BinaryDataIds: []string{id},
			IncludeBinary: false,
		})
		if err == nil {
			break
		}
	}

	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	data := resp.GetData()
	if len(data) != 1 {
		return errors.Errorf("expected a single response, received %d", len(data))
	}

	requestBody := BinaryMetadataToJSONLRequest{
		BinaryMetadata: []*datapb.BinaryMetadata{data[0].GetMetadata()},
		Path:           path,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal request")
	}

	url := fmt.Sprintf("%s/binary-metadata-to-jsonl", c.baseURL.String())
	req, err := http.NewRequestWithContext(c.c.Context, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrapf(err, "failed to create request")
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	token, ok := c.conf.Auth.(*token)
	if ok {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	}
	apiKey, ok := c.conf.Auth.(*apiKey)
	if ok {
		req.Header.Set("key_id", apiKey.KeyID)
		req.Header.Set("key", apiKey.KeyCrypto)
	}

	var res *http.Response
	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	for count := 0; count < maxRetryCount; count++ {
		res, err = httpClient.Do(req)
		if err == nil && res.StatusCode == http.StatusOK {
			debugf(c.c.App.Writer, args.Debug, "Binary metadata to JSONL request: attempt %d/%d succeeded", count+1, maxRetryCount)
			break
		}
		debugf(c.c.App.Writer, args.Debug, "Binary metadata to JSONL request: attempt %d/%d failed", count+1, maxRetryCount)
	}
	if err != nil {
		return errors.Wrapf(err, "error sending request")
	}
	if res.StatusCode != http.StatusOK {
		return errors.New(serverErrorMessage)
	}
	defer func() {
		utils.UncheckedError(res.Body.Close())
	}()

	jsonlData, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "error reading response")
	}

	_, err = file.Write(jsonlData)
	if err != nil {
		return errors.Wrapf(err, "error writing to file")
	}

	return nil
}

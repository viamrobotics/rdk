package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
	mlpb "go.viam.com/api/app/mltraining/v1"
	utilsml "go.viam.com/utils/machinelearning"
)

const (
	datasetFlagName           = "name"
	datasetFlagDatasetID      = "dataset-id"
	datasetFlagDatasetIDs     = "dataset-ids"
	dataFlagLocationID        = "location-id"
	dataFlagBinaryDataIDs     = "binary-data-ids"
	datasetFlagOnlyJSONLines  = "only-jsonl"
	datasetFlagForceLinuxPath = "force-linux-path"
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
	Destination    string
	DatasetID      string
	OnlyJSONl      bool
	ForceLinuxPath bool
	Parallel       uint
	Timeout        uint
}

// DatasetDownloadAction is the corresponding action for 'dataset export'.
func DatasetDownloadAction(c *cli.Context, args datasetDownloadArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.downloadDataset(args.Destination, args.DatasetID,
		args.OnlyJSONl, args.ForceLinuxPath, args.Parallel, args.Timeout); err != nil {
		return err
	}
	return nil
}

// downloadDataset downloads a dataset with the specified ID.
func (c *viamClient) downloadDataset(dst, datasetID string, onlyJSONLines, forceLinuxPath bool, parallelDownloads, timeout uint) error {
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
			datasetErr := binaryDataToJSONLines(c.c.Context, c.dataClient, datasetFilePath, datasetFile, id, forceLinuxPath)

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

func binaryDataToJSONLines(ctx context.Context, client datapb.DataServiceClient, dst string, file *os.File,
	id string, forceLinuxPath bool,
) error {
	var resp *datapb.BinaryDataByIDsResponse
	var err error
	for count := 0; count < maxRetryCount; count++ {
		resp, err = client.BinaryDataByIDs(ctx, &datapb.BinaryDataByIDsRequest{
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
	datum := data[0]

	fileName := filepath.Join(dst, filenameForDownload(datum.GetMetadata()))
	if forceLinuxPath {
		fileName = filepath.ToSlash(fileName)
	}
	ext := datum.GetMetadata().GetFileExt()
	// If the file is gzipped, unzip.
	if ext != gzFileExt && filepath.Ext(fileName) != ext {
		// If the file name did not already include the extension (e.g. for data capture files), add it.
		// Don't do this for files that we're unzipping.
		fileName += ext
	}

	imageMetadata := &utilsml.ImageMetadata{
		Timestamp:      datum.GetMetadata().GetTimeRequested().AsTime(),
		Tags:           datum.GetMetadata().GetCaptureMetadata().GetTags(),
		Annotations:    datum.GetMetadata().GetAnnotations(),
		Path:           fileName,
		BinaryDataID:   datum.GetMetadata().GetBinaryDataId(),
		OrganizationID: datum.GetMetadata().GetCaptureMetadata().GetOrganizationId(),
		LocationID:     datum.GetMetadata().GetCaptureMetadata().GetLocationId(),
		RobotID:        datum.GetMetadata().GetCaptureMetadata().GetRobotId(),
		PartID:         datum.GetMetadata().GetCaptureMetadata().GetPartId(),
		ComponentName:  datum.GetMetadata().GetCaptureMetadata().GetComponentName(),
	}
	_, err = utilsml.ImageMetadataToJSONLines([]*utilsml.ImageMetadata{imageMetadata}, nil, mlpb.ModelType_MODEL_TYPE_UNSPECIFIED, file)
	if err != nil {
		return errors.Wrap(err, "error writing to file")
	}
	return nil
}

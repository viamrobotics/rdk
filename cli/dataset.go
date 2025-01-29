package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	datapb "go.viam.com/api/app/data/v1"
	datasetpb "go.viam.com/api/app/dataset/v1"
)

const (
	datasetFlagName             = "name"
	datasetFlagDatasetID        = "dataset-id"
	datasetFlagDatasetIDs       = "dataset-ids"
	dataFlagLocationID          = "location-id"
	dataFlagFileIDs             = "file-ids"
	datasetFlagIncludeJSONLines = "include-jsonl"
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
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
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
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	_, err := c.datasetClient.RenameDataset(context.Background(),
		&datasetpb.RenameDatasetRequest{Id: datasetID, Name: newDatasetName})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	printf(c.c.App.Writer, "Dataset with ID %s renamed to %s", datasetID, newDatasetName)
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
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
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
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
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
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	_, err := c.datasetClient.DeleteDataset(context.Background(),
		&datasetpb.DeleteDatasetRequest{Id: datasetID})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	printf(c.c.App.Writer, "Dataset with ID %s deleted", datasetID)
	return nil
}

type datasetDownloadArgs struct {
	Destination  string
	DatasetID    string
	IncludeJSONl bool
	Parallel     uint
	Timeout      uint
}

// DatasetDownloadAction is the corresponding action for 'dataset download'.
func DatasetDownloadAction(c *cli.Context, args datasetDownloadArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.downloadDataset(args.Destination, args.DatasetID,
		args.IncludeJSONl, args.Parallel, args.Timeout); err != nil {
		return err
	}
	return nil
}

// downloadDataset downloads a dataset with the specified ID.
func (c *viamClient) downloadDataset(dst, datasetID string, includeJSONLines bool, parallelDownloads, timeout uint) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}

	var datasetFile *os.File
	var err error
	if includeJSONLines {
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
	}
	resp, err := c.datasetClient.ListDatasetsByIDs(context.Background(),
		&datasetpb.ListDatasetsByIDsRequest{Ids: []string{datasetID}})
	if err != nil {
		return errors.Wrapf(err, "error getting dataset ID")
	}
	if len(resp.GetDatasets()) == 0 {
		return fmt.Errorf("%s does not match any dataset IDs", datasetID)
	}

	return c.performActionOnBinaryDataFromFilter(
		func(id *datapb.BinaryID) error {
			downloadErr := c.downloadBinary(dst, id, timeout)
			var datasetErr error
			if includeJSONLines {
				datasetErr = binaryDataToJSONLines(c.c.Context, c.dataClient, dst, datasetFile, id)
			}
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
	id *datapb.BinaryID,
) error {
	var resp *datapb.BinaryDataByIDsResponse
	var err error
	for count := 0; count < maxRetryCount; count++ {
		resp, err = client.BinaryDataByIDs(ctx, &datapb.BinaryDataByIDsRequest{
			BinaryIds:     []*datapb.BinaryID{id},
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

	// Make JSONLines
	var jsonl interface{}

	annotations := []Annotation{}
	for _, tag := range datum.GetMetadata().GetCaptureMetadata().GetTags() {
		annotations = append(annotations, Annotation{AnnotationLabel: tag})
	}
	bboxAnnotations := convertBoundingBoxes(datum.GetMetadata().GetAnnotations().GetBboxes())

	fileName := filepath.Join(dst, dataDir, filenameForDownload(datum.GetMetadata()))
	ext := datum.GetMetadata().GetFileExt()
	// If the file is gzipped, unzip.
	if ext != gzFileExt && filepath.Ext(fileName) != ext {
		// If the file name did not already include the extension (e.g. for data capture files), add it.
		// Don't do this for files that we're unzipping.
		fileName += ext
	}

	jsonl = ImageMetadata{
		ImagePath:                 fileName,
		ClassificationAnnotations: annotations,
		BBoxAnnotations:           bboxAnnotations,
	}

	line, err := json.Marshal(jsonl)
	if err != nil {
		return errors.Wrap(err, "error formatting JSON")
	}
	line = append(line, "\n"...)
	_, err = file.Write(line)
	if err != nil {
		return errors.Wrap(err, "error writing to file")
	}

	return nil
}

func convertBoundingBoxes(protoBBoxes []*datapb.BoundingBox) []BBoxAnnotation {
	bboxes := make([]BBoxAnnotation, len(protoBBoxes))
	for i, box := range protoBBoxes {
		bboxes[i] = BBoxAnnotation{
			AnnotationLabel: box.GetLabel(),
			XMinNormalized:  box.GetXMinNormalized(),
			XMaxNormalized:  box.GetXMaxNormalized(),
			YMinNormalized:  box.GetYMinNormalized(),
			YMaxNormalized:  box.GetYMaxNormalized(),
		}
	}
	return bboxes
}

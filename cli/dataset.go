package cli

import (
	"context"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	datasetpb "go.viam.com/api/app/dataset/v1"
)

const (
	datasetFlagName       = "name"
	datasetFlagDatasetID  = "dataset-id"
	datasetFlagDatasetIDs = "dataset-ids"
	dataFlagLocationID    = "location-id"
	dataFlagFileIDs       = "file-ids"
)

// DatasetCreateAction is the corresponding action for 'dataset create'.
func DatasetCreateAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.createDataset(c.String(dataFlagOrgID), c.String(datasetFlagName)); err != nil {
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

// DatasetRenameAction is the corresponding action for 'dataset rename'.
func DatasetRenameAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.renameDataset(c.String(datasetFlagDatasetID), c.String(datasetFlagName)); err != nil {
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

// DatasetListAction is the corresponding action for 'dataset list'.
func DatasetListAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	datasetIDs := c.StringSlice(datasetFlagDatasetIDs)
	orgID := c.String(dataFlagOrgID)

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

// DatasetDeleteAction is the corresponding action for 'dataset rename'.
func DatasetDeleteAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}
	if err := client.deleteDataset(c.String(datasetFlagDatasetID)); err != nil {
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

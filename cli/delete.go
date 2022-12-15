package cli

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
)

// DeleteBinaryData deletes binary data matching filter.
func (c *AppClient) DeleteBinaryData(dst string, filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.sendDeleteBinaryDataByFilterRequest(filter)
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	deletedCount := resp.GetDeletedCount()
	status := resp.GetResult().GetStatus()

	switch status {
	case datapb.Status_STATUS_PARTIAL_SUCCESS:
		fmt.Fprint(c.c.App.Writer, "received errors when deleting objects\n", resp.GetResult().GetMessage())
	case datapb.Status_STATUS_SUCCESS:
		fmt.Fprintf(c.c.App.Writer, "deleted %d files", deletedCount)
	default:
		break
	}

	return nil
}

func (c *AppClient) sendDeleteBinaryDataByFilterRequest(filter *datapb.Filter) (*datapb.DeleteBinaryDataByFilterResponse, error) {
	return c.dataClient.DeleteBinaryDataByFilter(context.Background(), &datapb.DeleteBinaryDataByFilterRequest{
		Filter: filter,
	},
	)
}

func (c *AppClient) sendDeleteTabularDataByFilterRequest(filter *datapb.Filter) (*datapb.DeleteTabularDataByFilterResponse, error) {
	return c.dataClient.DeleteTabularDataByFilter(context.Background(), &datapb.DeleteTabularDataByFilterRequest{
		Filter: filter,
	},
	)
}

// TabularData delete tabular data matching filter.
func (c *AppClient) DeleteTabularData(dst string, filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.sendDeleteTabularDataByFilterRequest(filter)
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	deletedCount := resp.GetDeletedCount()
	status := resp.GetResult().GetStatus()

	switch status {
	case datapb.Status_STATUS_PARTIAL_SUCCESS:
		fmt.Fprint(c.c.App.Writer, "received errors when deleting objects\n", resp.GetResult().GetMessage())
	case datapb.Status_STATUS_SUCCESS:
		fmt.Fprintf(c.c.App.Writer, "deleted %d files", deletedCount)
	default:
		break
	}

	return nil
}

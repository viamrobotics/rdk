package cli

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
)

// DeleteBinaryData deletes binary data matching filter.
func (c *AppClient) DeleteBinaryData(filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.dataClient.DeleteBinaryDataByFilter(context.Background(),
		&datapb.DeleteBinaryDataByFilterRequest{Filter: filter})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	deletedCount := resp.GetDeletedCount()
	status := resp.GetResult().GetStatus()

	if status == datapb.Status_STATUS_PARTIAL_SUCCESS {
		fmt.Fprint(c.c.App.Writer, "received errors when deleting objects\n", resp.GetResult().GetMessage())
	}
	fmt.Fprintf(c.c.App.Writer, "deleted %d files\n", deletedCount)
	return nil
}

// DeleteTabularData delete tabular data matching filter.
func (c *AppClient) DeleteTabularData(filter *datapb.Filter) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	resp, err := c.dataClient.DeleteTabularDataByFilter(context.Background(),
		&datapb.DeleteTabularDataByFilterRequest{Filter: filter})
	if err != nil {
		return errors.Wrapf(err, "received error from server")
	}
	deletedCount := resp.GetDeletedCount()
	status := resp.GetResult().GetStatus()

	if status == datapb.Status_STATUS_PARTIAL_SUCCESS {
		fmt.Fprint(c.c.App.Writer, "received errors when deleting objects\n", resp.GetResult().GetMessage())
	}
	fmt.Fprintf(c.c.App.Writer, "deleted %d datapoints\n", deletedCount)
	return nil
}

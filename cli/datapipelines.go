package cli

import (
	"context"
	"errors"

	"github.com/urfave/cli/v2"
	datapipelinespb "go.viam.com/api/app/datapipelines/v1"
)

type datapipelineListArgs struct {
	OrgID string
}

func DatapipelineListAction(c *cli.Context, args datapipelineListArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("organization ID is required")
	}

	resp, err := client.datapipelinesClient.ListDataPipelines(context.Background(), &datapipelinespb.ListDataPipelinesRequest{
		OrganizationId: orgID,
	})
	if err != nil {
		return err
	}

	for _, pipeline := range resp.GetDataPipelines() {
		printf(c.App.Writer, "\t%s (ID: %s)", pipeline.Name, pipeline.Id)
	}

	return nil
}

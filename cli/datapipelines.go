package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/bson"
	datapipelinespb "go.viam.com/api/app/datapipelines/v1"
)

const (
	datapipelineFlagName     = "name"
	datapipelineFlagSchedule = "schedule"
	datapipelineFlagMQL      = "mql"
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

type datapipelineCreateArgs struct {
	OrgID    string
	Name     string
	Schedule string
	MQL      string
}

func DatapipelineCreateAction(c *cli.Context, args datapipelineCreateArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	orgID := args.OrgID
	if orgID == "" {
		return errors.New("organization ID is required")
	}

	name := args.Name
	if name == "" {
		return errors.New("data pipeline name is required")
	}

	schedule := args.Schedule
	if schedule == "" {
		return errors.New("data pipeline schedule is required")
	}

	// TODO: validate cron expression

	mql := args.MQL
	if mql == "" {
		return errors.New("data pipeline MQL is required")
	}

	// Parse the MQL stages directly into BSON
	// TODO: look into more leniant JSON parser
	var mqlArray []bson.M
	if err := bson.UnmarshalExtJSON([]byte(mql), false, &mqlArray); err != nil {
		return fmt.Errorf("invalid MQL: %w", err)
	}

	var mqlBinary [][]byte
	for _, stage := range mqlArray {
		bytes, err := bson.Marshal(stage)
		if err != nil {
			return fmt.Errorf("error converting MQL stage to BSON: %w", err)
		}
		mqlBinary = append(mqlBinary, bytes)
	}

	// TODO: support MQL file path

	resp, err := client.datapipelinesClient.CreateDataPipeline(context.Background(), &datapipelinespb.CreateDataPipelineRequest{
		OrganizationId: orgID,
		Name:           name,
		Schedule:       schedule,
		MqlBinary:      mqlBinary,
	})
	if err != nil {
		return err
	}

	printf(c.App.Writer, "%s (ID: %s) created", name, resp.GetId())

	return nil
}

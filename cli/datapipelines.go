package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/bson"
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

type datapipelineCreateArgs struct {
	OrgID    string
	Name     string
	Schedule string
	MQL      string
	MqlFile  string
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
	mqlFile := args.MqlFile

	if mqlFile != "" {
		if mql != "" {
			return errors.New("data pipeline MQL and MQL file cannot both be provided")
		}

		content, err := os.ReadFile(mqlFile)
		if err != nil {
			return fmt.Errorf("error reading MQL file: %w", err)
		}
		mql = string(content)
	}

	if mql == "" {
		return errors.New("missing data pipeline MQL")
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

	resp, err := client.datapipelinesClient.CreateDataPipeline(context.Background(), &datapipelinespb.CreateDataPipelineRequest{
		OrganizationId: orgID,
		Name:           name,
		Schedule:       schedule,
		MqlBinary:      mqlBinary,
	})
	if err != nil {
		return fmt.Errorf("error creating data pipeline: %w", err)
	}

	printf(c.App.Writer, "%s (ID: %s) created", name, resp.GetId())

	return nil
}

type datapipelineUpdateArgs struct {
	ID       string
	Name     string
	Schedule string
	MQL      string
	MqlFile  string
}

func DatapipelineUpdateAction(c *cli.Context, args datapipelineUpdateArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	id := args.ID
	if id == "" {
		return errors.New("data pipeline ID is required")
	}

	// TODO: maybe load existing pipeline and update fields?

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
	mqlFile := args.MqlFile

	if mqlFile != "" {
		if mql != "" {
			return errors.New("data pipeline MQL and MQL file cannot both be provided")
		}

		content, err := os.ReadFile(mqlFile)
		if err != nil {
			return fmt.Errorf("error reading MQL file: %w", err)
		}
		mql = string(content)
	}

	if mql == "" {
		return errors.New("missing data pipeline MQL")
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

	_, err = client.datapipelinesClient.UpdateDataPipeline(context.Background(), &datapipelinespb.UpdateDataPipelineRequest{
		Id:        id,
		Name:      name,
		Schedule:  schedule,
		MqlBinary: mqlBinary,
	})
	if err != nil {
		return fmt.Errorf("error updating data pipeline: %w", err)
	}

	printf(c.App.Writer, "%s (id: %s) updated", name, id)
	return nil
}

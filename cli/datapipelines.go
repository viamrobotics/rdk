package cli

import (
	"context"
	"encoding/json"
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

	resp, err := client.datapipelinesClient.ListDataPipelines(context.Background(), &datapipelinespb.ListDataPipelinesRequest{
		OrganizationId: args.OrgID,
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

	mqlBinary, err := parseMQL(args.MQL, args.MqlFile)
	if err != nil {
		return err
	}

	resp, err := client.datapipelinesClient.CreateDataPipeline(context.Background(), &datapipelinespb.CreateDataPipelineRequest{
		OrganizationId: args.OrgID,
		Name:           args.Name,
		Schedule:       args.Schedule,
		MqlBinary:      mqlBinary,
	})
	if err != nil {
		return fmt.Errorf("error creating data pipeline: %w", err)
	}

	printf(c.App.Writer, "%s (ID: %s) created.", args.Name, resp.GetId())

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

	resp, err := client.datapipelinesClient.GetDataPipeline(context.Background(), &datapipelinespb.GetDataPipelineRequest{
		Id: args.ID,
	})
	if err != nil {
		return fmt.Errorf("error getting data pipeline: %w", err)
	}
	current := resp.GetDataPipeline()

	name := args.Name
	if name == "" {
		name = current.GetName()
	}

	schedule := args.Schedule
	if schedule == "" {
		schedule = current.GetSchedule()
	}

	mqlBinary := current.GetMqlBinary()
	if args.MQL != "" || args.MqlFile != "" {
		mqlBinary, err = parseMQL(args.MQL, args.MqlFile)
		if err != nil {
			return err
		}
	}

	_, err = client.datapipelinesClient.UpdateDataPipeline(context.Background(), &datapipelinespb.UpdateDataPipelineRequest{
		Id:        args.ID,
		Name:      name,
		Schedule:  schedule,
		MqlBinary: mqlBinary,
	})
	if err != nil {
		return fmt.Errorf("error updating data pipeline: %w", err)
	}

	printf(c.App.Writer, "%s (id: %s) updated.", name, args.ID)
	return nil
}

type datapipelineDeleteArgs struct {
	ID string
}

func DatapipelineDeleteAction(c *cli.Context, args datapipelineDeleteArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	_, err = client.datapipelinesClient.DeleteDataPipeline(context.Background(), &datapipelinespb.DeleteDataPipelineRequest{
		Id: args.ID,
	})
	if err != nil {
		return fmt.Errorf("error deleting data pipeline: %w", err)
	}

	printf(c.App.Writer, "data pipeline (id: %s) deleted.", args.ID)
	return nil
}

type datapipelineDescribeArgs struct {
	ID string
}

func DatapipelineDescribeAction(c *cli.Context, args datapipelineDescribeArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	resp, err := client.datapipelinesClient.GetDataPipeline(context.Background(), &datapipelinespb.GetDataPipelineRequest{
		Id: args.ID,
	})
	if err != nil {
		return fmt.Errorf("error getting data pipeline: %w", err)
	}
	pipeline := resp.GetDataPipeline()

	printf(c.App.Writer, "ID: %s", pipeline.GetId())
	printf(c.App.Writer, "Name: %s", pipeline.GetName())
	printf(c.App.Writer, "Time window: %s", pipeline.GetSchedule())
	mql, err := mqlJSON(pipeline.GetMqlBinary())
	if err != nil {
		return fmt.Errorf("error getting MQL query: %w", err)
	}
	printf(c.App.Writer, "MQL query: %s", mql)
	// TODO: pending implementation of PipelineRuns API
	// printf(c.App.Writer, "Last execution: %s", pipeline.GetLastExecution())

	return nil
}

func parseMQL(mql, mqlFile string) ([][]byte, error) {
	if mqlFile != "" {
		if mql != "" {
			return nil, errors.New("data pipeline MQL and MQL file cannot both be provided")
		}

		content, err := os.ReadFile(mqlFile)
		if err != nil {
			return nil, fmt.Errorf("error reading MQL file: %w", err)
		}
		mql = string(content)
	}

	if mql == "" {
		return nil, errors.New("missing data pipeline MQL")
	}

	// Parse the MQL stages directly into BSON
	// TODO: look into more leniant JSON parser
	var mqlArray []bson.M
	if err := bson.UnmarshalExtJSON([]byte(mql), false, &mqlArray); err != nil {
		return nil, fmt.Errorf("invalid MQL: %w", err)
	}

	var mqlBinary [][]byte
	for _, stage := range mqlArray {
		bytes, err := bson.Marshal(stage)
		if err != nil {
			return nil, fmt.Errorf("error converting MQL stage to BSON: %w", err)
		}
		mqlBinary = append(mqlBinary, bytes)
	}

	return mqlBinary, nil
}

func mqlJSON(mql [][]byte) (string, error) {
	var stages []bson.M
	for _, bsonBytes := range mql {
		var stage bson.M
		if err := bson.Unmarshal(bsonBytes, &stage); err != nil {
			return "", fmt.Errorf("error unmarshaling BSON stage: %w", err)
		}
		stages = append(stages, stage)
	}

	jsonBytes, err := json.MarshalIndent(stages, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling stages to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

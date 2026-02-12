package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/yosuke-furukawa/json5/encoding/json5"
	"go.mongodb.org/mongo-driver/bson"
	pb "go.viam.com/api/app/data/v1"
	datapipelinespb "go.viam.com/api/app/datapipelines/v1"
)

// pipelineRunStatusMap maps pipeline run statuses to human-readable strings.
var pipelineRunStatusMap = map[datapipelinespb.DataPipelineRunStatus]string{
	datapipelinespb.DataPipelineRunStatus_DATA_PIPELINE_RUN_STATUS_UNSPECIFIED: "Unknown",
	datapipelinespb.DataPipelineRunStatus_DATA_PIPELINE_RUN_STATUS_SCHEDULED:   "Scheduled",
	datapipelinespb.DataPipelineRunStatus_DATA_PIPELINE_RUN_STATUS_STARTED:     "Running",
	datapipelinespb.DataPipelineRunStatus_DATA_PIPELINE_RUN_STATUS_COMPLETED:   "Success",
	datapipelinespb.DataPipelineRunStatus_DATA_PIPELINE_RUN_STATUS_FAILED:      "Failed",
}

// dataSourceTypeMap maps data source types to human-readable strings.
var dataSourceTypeMap = map[pb.TabularDataSourceType]string{
	pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_UNSPECIFIED:   "Unknown",
	pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_STANDARD:      "Standard",
	pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_HOT_STORAGE:   "Hot Storage",
	pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_PIPELINE_SINK: "Pipeline Sink",
}

// dataSourceType constants for data source types.
var (
	StandardDataSourceType   = "standard"
	HotStorageDataSourceType = "hotstorage"
)

type datapipelineListArgs struct {
	OrgID string
}

// DatapipelineListAction lists all data pipelines for an organization.
func DatapipelineListAction(c *cli.Context, args datapipelineListArgs) error {
	if args.OrgID == "" {
		return errors.New("must provide an organization ID to list data pipelines")
	}
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
		enabled := "Enabled"
		if !pipeline.Enabled {
			enabled = "Disabled"
		}
		dataSourceType := dataSourceTypeMap[*pipeline.DataSourceType]
		printf(c.App.Writer, "\t%s (ID: %s) [%s] [Data Source Type: %s]", pipeline.Name, pipeline.Id, enabled, dataSourceType)
	}

	return nil
}

type datapipelineCreateArgs struct {
	OrgID          string
	Name           string
	Schedule       string
	MQL            string
	MqlPath        string
	DataSourceType string
	EnableBackfill bool
}

// DatapipelineCreateAction creates a new data pipeline.
func DatapipelineCreateAction(c *cli.Context, args datapipelineCreateArgs) error {
	if args.OrgID == "" {
		return errors.New("must provide an organization ID to create a data pipeline")
	}
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	mqlBinary, err := parseMQL(args.MQL, args.MqlPath)
	if err != nil {
		return err
	}

	dataSourceType, err := dataSourceTypeToProto(args.DataSourceType)
	if err != nil {
		return err
	}

	resp, err := client.datapipelinesClient.CreateDataPipeline(context.Background(), &datapipelinespb.CreateDataPipelineRequest{
		OrganizationId: args.OrgID,
		Name:           args.Name,
		Schedule:       args.Schedule,
		MqlBinary:      mqlBinary,
		DataSourceType: &dataSourceType,
		EnableBackfill: &args.EnableBackfill,
	})
	if err != nil {
		return fmt.Errorf("error creating data pipeline: %w", err)
	}

	printf(c.App.Writer, "%s (ID: %s) created.", args.Name, resp.GetId())

	return nil
}

type datapipelineRenameArgs struct {
	ID   string
	Name string
}

// DatapipelineRenameAction renames an existing data pipeline.
func DatapipelineRenameAction(c *cli.Context, args datapipelineRenameArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	_, err = client.datapipelinesClient.RenameDataPipeline(context.Background(), &datapipelinespb.RenameDataPipelineRequest{
		Id:   args.ID,
		Name: args.Name,
	})
	if err != nil {
		return fmt.Errorf("error updating data pipeline: %w", err)
	}

	printf(c.App.Writer, "%s (id: %s) renamed.", args.Name, args.ID)
	return nil
}

type datapipelineDeleteArgs struct {
	ID string
}

// DatapipelineDeleteAction deletes a data pipeline.
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

// DatapipelineDescribeAction describes a data pipeline and its status.
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

	runsResp, err := client.datapipelinesClient.ListDataPipelineRuns(context.Background(), &datapipelinespb.ListDataPipelineRunsRequest{
		Id:       args.ID,
		PageSize: 1,
	})
	if err != nil {
		return fmt.Errorf("error getting list of pipeline runs: %w", err)
	}
	runs := runsResp.Runs

	mql, err := mqlJSON(pipeline.GetMqlBinary())
	if err != nil {
		warningf(c.App.Writer, "error parsing MQL query: %s", err)
		mql = "(error parsing MQL query)"
	}

	printf(c.App.Writer, "ID: %s", pipeline.GetId())
	printf(c.App.Writer, "Name: %s", pipeline.GetName())
	printf(c.App.Writer, "Enabled: %t", pipeline.GetEnabled())
	printf(c.App.Writer, "Schedule: %s", pipeline.GetSchedule())
	printf(c.App.Writer, "MQL query: %s", mql)
	printf(c.App.Writer, "DataSourceType: %s", pipeline.GetDataSourceType())

	if len(runs) > 0 {
		r := runs[0]

		printf(c.App.Writer, "Last run:")
		printf(c.App.Writer, "  Status: %s", pipelineRunStatusMap[r.GetStatus()])
		printf(c.App.Writer, "  Started: %s", r.GetStartTime().AsTime().Format(time.RFC3339))
		printf(c.App.Writer, "  Data range: [%s, %s]",
			r.GetDataStartTime().AsTime().Format(time.RFC3339),
			r.GetDataEndTime().AsTime().Format(time.RFC3339))
		if r.GetEndTime() != nil {
			printf(c.App.Writer, "  Ended: %s", r.GetEndTime().AsTime().Format(time.RFC3339))
		}
		if r.GetErrorMessage() != "" {
			printf(c.App.Writer, "  Error: %s", r.GetErrorMessage())
		}
	} else {
		printf(c.App.Writer, "Has not run yet.")
	}

	return nil
}

type datapipelineEnableArgs struct {
	ID string
}

// DatapipelineEnableAction enables a data pipeline.
func DatapipelineEnableAction(c *cli.Context, args datapipelineEnableArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	_, err = client.datapipelinesClient.EnableDataPipeline(context.Background(), &datapipelinespb.EnableDataPipelineRequest{
		Id: args.ID,
	})
	if err != nil {
		return fmt.Errorf("error enabling data pipeline: %w", err)
	}

	printf(c.App.Writer, "data pipeline (id: %s) enabled.", args.ID)
	return nil
}

type datapipelineDisableArgs struct {
	ID string
}

// DatapipelineDisableAction disables a data pipeline.
func DatapipelineDisableAction(c *cli.Context, args datapipelineDisableArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	_, err = client.datapipelinesClient.DisableDataPipeline(context.Background(), &datapipelinespb.DisableDataPipelineRequest{
		Id: args.ID,
	})
	if err != nil {
		return fmt.Errorf("error disabling data pipeline: %w", err)
	}

	printf(c.App.Writer, "data pipeline (id: %s) disabled.", args.ID)
	return nil
}

func parseMQL(mql, mqlFile string) ([][]byte, error) {
	if mqlFile != "" && mql != "" {
		return nil, errors.New("data pipeline MQL and MQL file cannot both be provided")
	}

	if mqlFile != "" {
		//nolint:gosec // mqlFile is a user-provided path for reading MQL query files
		content, err := os.ReadFile(mqlFile)
		if err != nil {
			return nil, fmt.Errorf("error reading MQL file: %w", err)
		}
		mql = string(content)
	}

	if mql == "" {
		return nil, errors.New("missing data pipeline MQL")
	}

	// Parse the MQL stages JSON (using JSON5 for unquoted keys + comments).
	var mqlArray []bson.M
	if err := json5.Unmarshal([]byte(mql), &mqlArray); err != nil {
		return nil, fmt.Errorf("unable to parse MQL argument: %w", err)
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

func dataSourceTypeToProto(dataSourceType string) (pb.TabularDataSourceType, error) {
	switch dataSourceType {
	case StandardDataSourceType:
		return pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_STANDARD, nil
	case HotStorageDataSourceType:
		return pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_HOT_STORAGE, nil
	default:
		return pb.TabularDataSourceType_TABULAR_DATA_SOURCE_TYPE_UNSPECIFIED,
			fmt.Errorf("invalid data source type: %s. Supported values: [%s, %s]",
				dataSourceType,
				StandardDataSourceType,
				HotStorageDataSourceType,
			)
	}
}

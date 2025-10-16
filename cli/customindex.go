package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
	pb "go.viam.com/api/app/data/v1"
)

const (
	hotStoreCollectionType     = pb.IndexableCollection_INDEXABLE_COLLECTION_HOT_STORE
	pipelineSinkCollectionType = pb.IndexableCollection_INDEXABLE_COLLECTION_PIPELINE_SINK
	unspecifiedCollectionType  = pb.IndexableCollection_INDEXABLE_COLLECTION_UNSPECIFIED
)

var (
	ErrInvalidCollectionType  = errors.New("invalid collection type, must be one of: hot_store, pipeline_sink")
	ErrPipelineNameRequired   = errors.New("--pipeline-name is required when --collection-type is 'pipeline_sink'")
	ErrPipelineNameNotAllowed = errors.New("--pipeline-name can only be used when --collection-type is 'pipeline_sink'")
)

type createCustomIndexArgs struct {
	OrgID          string
	CollectionType string
	PipelineName   string
	IndexSpecPath  string
}

func CreateCustomIndexAction(c *cli.Context, args createCustomIndexArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	collectionType, err := validateCollectionTypeArgs(c, args.CollectionType)
	if err != nil {
		return err
	}

	indexSpec, err := readJSONToByteSlices(args.IndexSpecPath)
	if err != nil {
		return fmt.Errorf("failed to read index spec from file: %w", err)
	}

	_, err = client.dataClient.CreateIndex(context.Background(), &pb.CreateIndexRequest{
		OrganizationId: args.OrgID,
		CollectionType: collectionType,
		PipelineName:   &args.PipelineName,
		IndexSpec:      indexSpec,
	})
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	printf(c.App.Writer, "Create index request sent successfully")

	return nil
}

type deleteCustomIndexArgs struct {
	OrgID          string
	CollectionType string
	PipelineName   string
	IndexName      string
}

func DeleteCustomIndexAction(c *cli.Context, args deleteCustomIndexArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	collectionType, err := validateCollectionTypeArgs(c, args.CollectionType)
	if err != nil {
		return err
	}

	_, err = client.dataClient.DeleteIndex(context.Background(), &pb.DeleteIndexRequest{
		OrganizationId: args.OrgID,
		CollectionType: collectionType,
		PipelineName:   &args.PipelineName,
		IndexName:      args.IndexName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}

	printf(c.App.Writer, "Index (name: %s) deleted successfully", args.IndexName)

	return nil
}

func DeleteCustomIndexConfirmation(c *cli.Context, args deleteCustomIndexArgs) error {
	printf(c.App.Writer, "Are you sure you want to delete index (name: %s)? This action cannot be undone. (y/N): ", args.IndexName)
	if err := c.Err(); err != nil {
		return err
	}

	rawInput, err := bufio.NewReader(c.App.Reader).ReadString('\n')
	if err != nil {
		return err
	}

	if input := strings.ToUpper(strings.TrimSpace(rawInput)); input != "Y" {
		return errors.New("aborted")
	}
	return nil
}

type listCustomIndexesArgs struct {
	OrgID          string
	CollectionType string
	PipelineName   string
}

func ListCustomIndexesAction(c *cli.Context, args listCustomIndexesArgs) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	collectionType, err := validateCollectionTypeArgs(c, args.CollectionType)
	if err != nil {
		return err
	}

	resp, err := client.dataClient.ListIndexes(context.Background(), &pb.ListIndexesRequest{
		OrganizationId: args.OrgID,
		CollectionType: collectionType,
		PipelineName:   &args.PipelineName,
	})
	if err != nil {
		return fmt.Errorf("failed to list indexes: %w", err)
	}

	if len(resp.Indexes) == 0 {
		printf(c.App.Writer, "No indexes found")
		return nil
	}

	printf(c.App.Writer, "Indexes:\n")
	for _, index := range resp.Indexes {
		printf(c.App.Writer, "- Name: %s\n", index.IndexName)
		printf(c.App.Writer, "  Spec: %s\n", index.IndexSpec)

	}

	return nil
}

func validateCollectionTypeArgs(c *cli.Context, collectionType string) (pb.IndexableCollection, error) {
	var collectionTypeProto pb.IndexableCollection
	switch collectionType {
	case "hot_store":
		collectionTypeProto = hotStoreCollectionType
	case "pipeline_sink":
		collectionTypeProto = pipelineSinkCollectionType
	default:
		return unspecifiedCollectionType, ErrInvalidCollectionType
	}

	collectionTypeFlag := c.String(dataFlagCollectionType)
	pipelineName := c.String(dataFlagPipelineName)

	if collectionTypeFlag == "pipeline_sink" && pipelineName == "" {
		return unspecifiedCollectionType, ErrPipelineNameRequired
	}

	if collectionTypeFlag != "pipeline_sink" && pipelineName != "" {
		return unspecifiedCollectionType, ErrPipelineNameNotAllowed
	}

	return collectionTypeProto, nil
}

func readJSONToByteSlices(filePath string) ([][]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var rawMessages []json.RawMessage
	if err = json.Unmarshal(data, &rawMessages); err != nil {
		return nil, err
	}

	result := make([][]byte, len(rawMessages))
	for i, raw := range rawMessages {
		result[i] = []byte(raw)
	}

	return result, nil
}

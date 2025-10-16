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

	hotStoreCollectionTypeStr     = "hot_store"
	pipelineSinkCollectionTypeStr = "pipeline_sink"
)

var (
	// ErrInvalidCollectionType is returned when an invalid collection type is provided in the arguments.
	ErrInvalidCollectionType = errors.New("invalid collection type, must be one of: hot_store, pipeline_sink")

	// ErrPipelineNameRequired is returned when --pipeline-name is missing for pipeline_sink collection type.
	ErrPipelineNameRequired = errors.New("--pipeline-name is required when --collection-type is 'pipeline_sink'")

	// ErrPipelineNameNotAllowed is returned when --pipeline-name is provided for hot_store collection type.
	ErrPipelineNameNotAllowed = errors.New("--pipeline-name can only be used when --collection-type is 'pipeline_sink'")
)

type createCustomIndexArgs struct {
	OrgID          string
	CollectionType string
	PipelineName   string
	IndexSpecPath  string
}

// CreateCustomIndexAction creates a custom index for a specified organization and collection type
// using the provided index specification file in the arguments.
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

// DeleteCustomIndexAction deletes a custom index for a specified organization and collection type using the provided index name.
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

// DeleteCustomIndexConfirmation prompts the user for confirmation before deleting a custom index.
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

// ListCustomIndexesAction lists all custom indexes for a specified organization and collection type.
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
	case hotStoreCollectionTypeStr:
		collectionTypeProto = hotStoreCollectionType
	case pipelineSinkCollectionTypeStr:
		collectionTypeProto = pipelineSinkCollectionType
	default:
		return unspecifiedCollectionType, ErrInvalidCollectionType
	}

	collectionTypeFlag := c.String(dataFlagCollectionType)
	pipelineName := c.String(dataFlagPipelineName)

	if collectionTypeFlag == pipelineSinkCollectionTypeStr && pipelineName == "" {
		return unspecifiedCollectionType, ErrPipelineNameRequired
	}

	if collectionTypeFlag != pipelineSinkCollectionTypeStr && pipelineName != "" {
		return unspecifiedCollectionType, ErrPipelineNameNotAllowed
	}

	return collectionTypeProto, nil
}

func readJSONToByteSlices(filePath string) ([][]byte, error) {
	//nolint:gosec // filePath is a user-provided path for a JSON file containing an index spec
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

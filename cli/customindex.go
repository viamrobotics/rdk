package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
	pb "go.viam.com/api/app/data/v1"
)

const (
	hotStoreCollectionType     = pb.IndexableCollection_INDEXABLE_COLLECTION_HOT_STORE
	pipelineSinkCollectionType = pb.IndexableCollection_INDEXABLE_COLLECTION_PIPELINE_SINK
	unspecifiedCollectionType  = pb.IndexableCollection_INDEXABLE_COLLECTION_UNSPECIFIED

	hotStoreCollectionTypeStr     = "hot-storage"
	pipelineSinkCollectionTypeStr = "pipeline-sink"
)

var (
	errInvalidCollectionType  = errors.New("invalid collection type, must be one of: hot-storage, pipeline-sink")
	errPipelineNameRequired   = errors.New("--pipeline-name is required when --collection-type is 'pipeline-sink'")
	errPipelineNameNotAllowed = errors.New("--pipeline-name can only be used when --collection-type is 'pipeline-sink'")
)

type createCustomIndexArgs struct {
	OrgID          string
	CollectionType string
	PipelineName   string
	IndexSpecPath  string
}

// CreateCustomIndexAction creates a custom index for a specified organization and collection type
// using the provided index specification file in the arguments.
func CreateCustomIndexAction(ctx context.Context, cmd *cli.Command, args createCustomIndexArgs) error {
	if args.OrgID == "" {
		return errors.New("must provide an organization ID to create a custom index")
	}
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	collectionType, err := validateCollectionTypeArgs(cmd, args.CollectionType)
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

	printf(cmd.Root().Writer, "Create index request sent successfully")

	return nil
}

type deleteCustomIndexArgs struct {
	OrgID          string
	CollectionType string
	PipelineName   string
	IndexName      string
}

// DeleteCustomIndexAction deletes a custom index for a specified organization and collection type using the provided index name.
func DeleteCustomIndexAction(ctx context.Context, cmd *cli.Command, args deleteCustomIndexArgs) error {
	if args.OrgID == "" {
		return errors.New("must provide an organization ID to delete a custom index")
	}
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	collectionType, err := validateCollectionTypeArgs(cmd, args.CollectionType)
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

	printf(cmd.Root().Writer, "Index (name: %s) deleted successfully", args.IndexName)

	return nil
}

type listCustomIndexesArgs struct {
	OrgID          string
	CollectionType string
	PipelineName   string
}

// ListCustomIndexesAction lists all custom indexes for a specified organization and collection type.
func ListCustomIndexesAction(ctx context.Context, cmd *cli.Command, args listCustomIndexesArgs) error {
	if args.OrgID == "" {
		return errors.New("must provide an organization ID to list custom indexes")
	}
	client, err := newViamClient(ctx, cmd)
	if err != nil {
		return err
	}

	collectionType, err := validateCollectionTypeArgs(cmd, args.CollectionType)
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
		printf(cmd.Root().Writer, "No indexes found")
		return nil
	}

	printf(cmd.Root().Writer, "Indexes:\n")
	for _, index := range resp.Indexes {
		printf(cmd.Root().Writer, "- Name: %s\n", index.IndexName)
		printf(cmd.Root().Writer, "  Spec: %s\n", index.IndexSpec)
	}

	return nil
}

func validateCollectionTypeArgs(cmd *cli.Command, collectionType string) (pb.IndexableCollection, error) {
	var collectionTypeProto pb.IndexableCollection
	switch collectionType {
	case hotStoreCollectionTypeStr:
		collectionTypeProto = hotStoreCollectionType
	case pipelineSinkCollectionTypeStr:
		collectionTypeProto = pipelineSinkCollectionType
	default:
		return unspecifiedCollectionType, errInvalidCollectionType
	}

	collectionTypeFlag := cmd.String(dataFlagCollectionType)
	pipelineName := cmd.String(dataFlagPipelineName)

	if collectionTypeFlag == pipelineSinkCollectionTypeStr && pipelineName == "" {
		return unspecifiedCollectionType, errPipelineNameRequired
	}

	if collectionTypeFlag != pipelineSinkCollectionTypeStr && pipelineName != "" {
		return unspecifiedCollectionType, errPipelineNameNotAllowed
	}

	return collectionTypeProto, nil
}

func readJSONToByteSlices(filePath string) ([][]byte, error) {
	//nolint:gosec // filePath is a user-provided path for a JSON file containing an index spec
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var indexSpec struct {
		Key     json.RawMessage `json:"key"`
		Options json.RawMessage `json:"options,omitempty"`
	}

	if err = json.Unmarshal(data, &indexSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if len(indexSpec.Key) == 0 {
		return nil, fmt.Errorf("missing required 'key' field in index spec")
	}

	result := make([][]byte, 0, 2)
	result = append(result, []byte(indexSpec.Key))

	if len(indexSpec.Options) > 0 {
		result = append(result, []byte(indexSpec.Options))
	}

	return result, nil
}

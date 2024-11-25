package inject

import (
	"context"

	datasetpb "go.viam.com/api/app/dataset/v1"
	"google.golang.org/grpc"
)

// DatasetServiceClient represents a fake instance of a dataset service client.
type DatasetServiceClient struct {
	datasetpb.DatasetServiceClient
	CreateDatasetFunc func(ctx context.Context, in *datasetpb.CreateDatasetRequest,
		opts ...grpc.CallOption) (*datasetpb.CreateDatasetResponse, error)
	DeleteDatasetFunc func(ctx context.Context, in *datasetpb.DeleteDatasetRequest,
		opts ...grpc.CallOption) (*datasetpb.DeleteDatasetResponse, error)
	RenameDatasetFunc func(ctx context.Context, in *datasetpb.RenameDatasetRequest,
		opts ...grpc.CallOption) (*datasetpb.RenameDatasetResponse, error)
	ListDatasetsByOrganizationIDFunc func(ctx context.Context, in *datasetpb.ListDatasetsByOrganizationIDRequest,
		opts ...grpc.CallOption) (*datasetpb.ListDatasetsByOrganizationIDResponse, error)
	ListDatasetsByIDsFunc func(ctx context.Context, in *datasetpb.ListDatasetsByIDsRequest,
		opts ...grpc.CallOption) (*datasetpb.ListDatasetsByIDsResponse, error)
}

// CreateDataset calls the injected CreateDataset or the real version.
func (client *DatasetServiceClient) CreateDataset(ctx context.Context, in *datasetpb.CreateDatasetRequest,
	opts ...grpc.CallOption,
) (*datasetpb.CreateDatasetResponse, error) {
	if client.CreateDatasetFunc == nil {
		return client.DatasetServiceClient.CreateDataset(ctx, in, opts...)
	}
	return client.CreateDatasetFunc(ctx, in, opts...)
}

// DeleteDataset calls the injected DeleteDataset or the real version.
func (client *DatasetServiceClient) DeleteDataset(ctx context.Context, in *datasetpb.DeleteDatasetRequest,
	opts ...grpc.CallOption,
) (*datasetpb.DeleteDatasetResponse, error) {
	if client.DeleteDatasetFunc == nil {
		return client.DatasetServiceClient.DeleteDataset(ctx, in, opts...)
	}
	return client.DeleteDatasetFunc(ctx, in, opts...)
}

// RenameDataset calls the injected RenameDataset or the real version.
func (client *DatasetServiceClient) RenameDataset(ctx context.Context, in *datasetpb.RenameDatasetRequest,
	opts ...grpc.CallOption,
) (*datasetpb.RenameDatasetResponse, error) {
	if client.RenameDatasetFunc == nil {
		return client.DatasetServiceClient.RenameDataset(ctx, in, opts...)
	}
	return client.RenameDatasetFunc(ctx, in, opts...)
}

// ListDatasetsByOrganizationID calls the injected ListDatasetsByOrganizationID or the real version.
func (client *DatasetServiceClient) ListDatasetsByOrganizationID(ctx context.Context, in *datasetpb.ListDatasetsByOrganizationIDRequest,
	opts ...grpc.CallOption,
) (*datasetpb.ListDatasetsByOrganizationIDResponse, error) {
	if client.ListDatasetsByOrganizationIDFunc == nil {
		return client.DatasetServiceClient.ListDatasetsByOrganizationID(ctx, in, opts...)
	}
	return client.ListDatasetsByOrganizationIDFunc(ctx, in, opts...)
}

// ListDatasetsByIDs calls the injected ListDatasetsByIDs or the real version.
func (client *DatasetServiceClient) ListDatasetsByIDs(ctx context.Context, in *datasetpb.ListDatasetsByIDsRequest,
	opts ...grpc.CallOption,
) (*datasetpb.ListDatasetsByIDsResponse, error) {
	if client.ListDatasetsByIDsFunc == nil {
		return client.DatasetServiceClient.ListDatasetsByIDs(ctx, in, opts...)
	}
	return client.ListDatasetsByIDsFunc(ctx, in, opts...)
}

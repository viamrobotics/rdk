package inject

import (
	"context"

	datapipelinesPb "go.viam.com/api/app/datapipelines/v1"
	"google.golang.org/grpc"
)

// DataPipelinesServiceClient represents a fake instance of a data pipelines service client.
type DataPipelinesServiceClient struct {
	datapipelinesPb.DataPipelinesServiceClient
	ListDataPipelinesFunc func(ctx context.Context, in *datapipelinesPb.ListDataPipelinesRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.ListDataPipelinesResponse, error)
	GetDataPipelineFunc func(ctx context.Context, in *datapipelinesPb.GetDataPipelineRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.GetDataPipelineResponse, error)
	CreateDataPipelineFunc func(ctx context.Context, in *datapipelinesPb.CreateDataPipelineRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.CreateDataPipelineResponse, error)
	RenameDataPipelineFunc func(ctx context.Context, in *datapipelinesPb.RenameDataPipelineRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.RenameDataPipelineResponse, error)
	DeleteDataPipelineFunc func(ctx context.Context, in *datapipelinesPb.DeleteDataPipelineRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.DeleteDataPipelineResponse, error)
	EnableDataPipelineFunc func(ctx context.Context, in *datapipelinesPb.EnableDataPipelineRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.EnableDataPipelineResponse, error)
	DisableDataPipelineFunc func(ctx context.Context, in *datapipelinesPb.DisableDataPipelineRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.DisableDataPipelineResponse, error)
	ListDataPipelineRunsFunc func(ctx context.Context, in *datapipelinesPb.ListDataPipelineRunsRequest,
		opts ...grpc.CallOption) (*datapipelinesPb.ListDataPipelineRunsResponse, error)
}

// ListDataPipelines calls the injected ListDataPipelines or the real version.
func (client *DataPipelinesServiceClient) ListDataPipelines(ctx context.Context, in *datapipelinesPb.ListDataPipelinesRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.ListDataPipelinesResponse, error) {
	if client.ListDataPipelinesFunc == nil {
		return client.DataPipelinesServiceClient.ListDataPipelines(ctx, in, opts...)
	}
	return client.ListDataPipelinesFunc(ctx, in, opts...)
}

// GetDataPipeline calls the injected GetDataPipeline or the real version.
func (client *DataPipelinesServiceClient) GetDataPipeline(ctx context.Context, in *datapipelinesPb.GetDataPipelineRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.GetDataPipelineResponse, error) {
	if client.GetDataPipelineFunc == nil {
		return client.DataPipelinesServiceClient.GetDataPipeline(ctx, in, opts...)
	}
	return client.GetDataPipelineFunc(ctx, in, opts...)
}

// CreateDataPipeline calls the injected CreateDataPipeline or the real version.
func (client *DataPipelinesServiceClient) CreateDataPipeline(ctx context.Context, in *datapipelinesPb.CreateDataPipelineRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.CreateDataPipelineResponse, error) {
	if client.CreateDataPipelineFunc == nil {
		return client.DataPipelinesServiceClient.CreateDataPipeline(ctx, in, opts...)
	}
	return client.CreateDataPipelineFunc(ctx, in, opts...)
}

// RenameDataPipeline calls the injected RenameDataPipeline or the real version.
func (client *DataPipelinesServiceClient) RenameDataPipeline(ctx context.Context, in *datapipelinesPb.RenameDataPipelineRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.RenameDataPipelineResponse, error) {
	if client.RenameDataPipelineFunc == nil {
		return client.DataPipelinesServiceClient.RenameDataPipeline(ctx, in, opts...)
	}
	return client.RenameDataPipelineFunc(ctx, in, opts...)
}

// DeleteDataPipeline calls the injected DeleteDataPipeline or the real version.
func (client *DataPipelinesServiceClient) DeleteDataPipeline(ctx context.Context, in *datapipelinesPb.DeleteDataPipelineRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.DeleteDataPipelineResponse, error) {
	if client.DeleteDataPipelineFunc == nil {
		return client.DataPipelinesServiceClient.DeleteDataPipeline(ctx, in, opts...)
	}
	return client.DeleteDataPipelineFunc(ctx, in, opts...)
}

// EnableDataPipeline calls the injected EnableDataPipeline or the real version.
func (client *DataPipelinesServiceClient) EnableDataPipeline(ctx context.Context, in *datapipelinesPb.EnableDataPipelineRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.EnableDataPipelineResponse, error) {
	if client.EnableDataPipelineFunc == nil {
		return client.DataPipelinesServiceClient.EnableDataPipeline(ctx, in, opts...)
	}
	return client.EnableDataPipelineFunc(ctx, in, opts...)
}

// DisableDataPipeline calls the injected DisableDataPipeline or the real version.
func (client *DataPipelinesServiceClient) DisableDataPipeline(ctx context.Context, in *datapipelinesPb.DisableDataPipelineRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.DisableDataPipelineResponse, error) {
	if client.DisableDataPipelineFunc == nil {
		return client.DataPipelinesServiceClient.DisableDataPipeline(ctx, in, opts...)
	}
	return client.DisableDataPipelineFunc(ctx, in, opts...)
}

// ListDataPipelineRuns calls the injected ListDataPipelineRuns or the real version.
func (client *DataPipelinesServiceClient) ListDataPipelineRuns(ctx context.Context, in *datapipelinesPb.ListDataPipelineRunsRequest,
	opts ...grpc.CallOption,
) (*datapipelinesPb.ListDataPipelineRunsResponse, error) {
	if client.ListDataPipelineRunsFunc == nil {
		return client.DataPipelinesServiceClient.ListDataPipelineRuns(ctx, in, opts...)
	}
	return client.ListDataPipelineRunsFunc(ctx, in, opts...)
}

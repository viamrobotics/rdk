package inject

import (
	"context"

	datapb "go.viam.com/api/app/data/v1"
	"google.golang.org/grpc"
)

// DataServiceClient represents a fake instance of a data service client.
type DataServiceClient struct {
	datapb.DataServiceClient
	TabularDataByFilterFunc func(
		ctx context.Context,
		in *datapb.TabularDataByFilterRequest,
		opts ...grpc.CallOption,
	) (*datapb.TabularDataByFilterResponse, error)
}

// TabularDataByFilter calls the injected TabularDataByFilter or the real version.
func (client *DataServiceClient) TabularDataByFilter(ctx context.Context, in *datapb.TabularDataByFilterRequest, opts ...grpc.CallOption,
) (*datapb.TabularDataByFilterResponse, error) {
	if client.TabularDataByFilterFunc == nil {
		return client.DataServiceClient.TabularDataByFilter(ctx, in, opts...)
	}
	return client.TabularDataByFilterFunc(ctx, in, opts...)
}

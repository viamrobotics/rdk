package inject

import (
	"context"

	datapb "go.viam.com/api/app/datasync/v1"
	"google.golang.org/grpc"
)

// DataServiceClient represents a fake instance of a data service client.
type DataSyncServiceClient struct {
	datapb.DataSyncServiceClient
	DataCaptureUploadFunc func(ctx context.Context, in *datapb.DataCaptureUploadRequest,
		opts ...grpc.CallOption) (*datapb.DataCaptureUploadResponse, error)
	FileUploadFunc func(ctx context.Context,
		opts ...grpc.CallOption) (datapb.DataSyncService_FileUploadClient, error)
	StreamingDataCaptureUploadFunc func(ctx context.Context,
		opts ...grpc.CallOption) (datapb.DataSyncService_StreamingDataCaptureUploadClient, error)
}

// DataCaptureUpload uploads the contents and metadata for tabular data.
func (client *DataSyncServiceClient) DataCaptureUpload(ctx context.Context, in *datapb.DataCaptureUploadRequest,
	opts ...grpc.CallOption,
) (*datapb.DataCaptureUploadResponse, error) {
	if client.DataCaptureUploadFunc == nil {
		return client.DataSyncServiceClient.DataCaptureUpload(ctx, in, opts...)
	}
	return client.DataCaptureUploadFunc(ctx, in, opts...)
}

// FileUpload uploads the contents and metadata for binary (image + file) data,
// where the first packet must be the UploadMetadata.
func (client *DataSyncServiceClient) FileUpload(ctx context.Context,
	opts ...grpc.CallOption,
) (datapb.DataSyncService_FileUploadClient, error) {
	if client.FileUploadFunc == nil {
		return client.DataSyncServiceClient.FileUpload(ctx, opts...)
	}
	return client.FileUploadFunc(ctx, opts...)
}

// DataCaptureUpload uploads the contents and metadata for tabular data.
func (client *DataSyncServiceClient) StreamingDataCaptureUpload(ctx context.Context,
	opts ...grpc.CallOption,
) (datapb.DataSyncService_StreamingDataCaptureUploadClient, error) {
	if client.StreamingDataCaptureUploadFunc == nil {
		return client.DataSyncServiceClient.StreamingDataCaptureUpload(ctx, opts...)
	}
	return client.StreamingDataCaptureUpload(ctx, opts...)
}

type MockStreamingDataCaptureUploadClient struct {
	SendFunc         func(req *datapb.StreamingDataCaptureUploadRequest) error
	CloseAndRecvFunc func() (*datapb.StreamingDataCaptureUploadResponse, error)
}

func (m *MockStreamingDataCaptureUploadClient) Send(req *datapb.StreamingDataCaptureUploadRequest) error {
	return m.SendFunc(req)
}

func (m *MockStreamingDataCaptureUploadClient) CloseAndRecv() (*datapb.StreamingDataCaptureUploadResponse, error) {
	return m.CloseAndRecvFunc()
}

func (m *MockStreamingDataCaptureUploadClient) Context() context.Context {
	return context.Background()
}

// func (m *MockStreamingDataCaptureUploadClient) Header() (metadata.MD, error) {
// 	return nil, nil
// }

// func (m *MockStreamingDataCaptureUploadClient) Trailer() metadata.MD {
// 	return nil
// }

func (m *MockStreamingDataCaptureUploadClient) CloseSend() error {
	return nil
}

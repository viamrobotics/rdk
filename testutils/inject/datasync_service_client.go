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
	return client.StreamingDataCaptureUploadFunc(ctx, opts...)
}

type DataSyncService_StreamingDataCaptureUploadClient struct {
	datapb.DataSyncService_StreamingDataCaptureUploadClient
	SendFunc         func(*datapb.StreamingDataCaptureUploadRequest) error
	CloseAndRecvFunc func() (*datapb.StreamingDataCaptureUploadResponse, error)
	// grpc.ClientStream
}

func (client *DataSyncService_StreamingDataCaptureUploadClient) Send(req *datapb.StreamingDataCaptureUploadRequest) error {
	if client.SendFunc == nil {
		return client.DataSyncService_StreamingDataCaptureUploadClient.Send(req)
	}
	//test that the data we send is equal to what we expect
	return client.SendFunc(req)
}

func (client *DataSyncService_StreamingDataCaptureUploadClient) CloseAndRecv() (*datapb.StreamingDataCaptureUploadResponse, error) {
	if client.CloseAndRecvFunc == nil {
		return client.DataSyncService_StreamingDataCaptureUploadClient.CloseAndRecv()
	}
	return client.CloseAndRecvFunc()

}

type DataSyncService_FileUploadClient struct{
	datapb.DataSyncService_FileUploadClient
	SendFunc func(*datapb.FileUploadRequest) error
    CloseAndRecvFunc func() (*datapb.FileUploadResponse, error)
    // grpc.ClientStream
}
func (client *DataSyncService_FileUploadClient) Send(req *datapb.FileUploadRequest) error {
	if client.SendFunc == nil {
		return client.DataSyncService_FileUploadClient.Send(req)
	}
	//test that the data we send is equal to what we expect
	return client.SendFunc(req)
}

func (client *DataSyncService_FileUploadClient) CloseAndRecv() (*datapb.FileUploadResponse, error) {
	if client.CloseAndRecvFunc == nil {
		return client.DataSyncService_FileUploadClient.CloseAndRecv()
	}
	return client.CloseAndRecvFunc()

}
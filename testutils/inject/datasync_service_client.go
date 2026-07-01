package inject

import (
	"context"

	"braces.dev/errtrace"
	datapb "go.viam.com/api/app/datasync/v1"
	"google.golang.org/grpc"
)

// DataSyncServiceClient represents a fake instance of a data sync service client.
type DataSyncServiceClient struct {
	datapb.DataSyncServiceClient
	DataCaptureUploadFunc func(ctx context.Context, in *datapb.DataCaptureUploadRequest,
		opts ...grpc.CallOption) (*datapb.DataCaptureUploadResponse, error)
	FileUploadFunc func(ctx context.Context,
		opts ...grpc.CallOption) (datapb.DataSyncService_FileUploadClient, error)
	StreamingDataCaptureUploadFunc func(ctx context.Context,
		opts ...grpc.CallOption) (datapb.DataSyncService_StreamingDataCaptureUploadClient, error)
}

// DataCaptureUpload calls the injected DataCaptureUpload or the real version.
func (client *DataSyncServiceClient) DataCaptureUpload(ctx context.Context, in *datapb.DataCaptureUploadRequest,
	opts ...grpc.CallOption,
) (*datapb.DataCaptureUploadResponse, error) {
	if client.DataCaptureUploadFunc == nil {
		return errtrace.Wrap2(client.DataSyncServiceClient.DataCaptureUpload(ctx, in, opts...))
	}
	return errtrace.Wrap2(client.DataCaptureUploadFunc(ctx, in, opts...))
}

// FileUpload calls the injected FileUpload or the real version.
func (client *DataSyncServiceClient) FileUpload(ctx context.Context,
	opts ...grpc.CallOption,
) (datapb.DataSyncService_FileUploadClient, error) {
	if client.FileUploadFunc == nil {
		return errtrace.Wrap2(client.DataSyncServiceClient.FileUpload(ctx, opts...))
	}
	return errtrace.Wrap2(client.FileUploadFunc(ctx, opts...))
}

// StreamingDataCaptureUpload calls the injected StreamingDataCaptureUpload or the real version.
func (client *DataSyncServiceClient) StreamingDataCaptureUpload(ctx context.Context,
	opts ...grpc.CallOption,
) (datapb.DataSyncService_StreamingDataCaptureUploadClient, error) {
	if client.StreamingDataCaptureUploadFunc == nil {
		return errtrace.Wrap2(client.DataSyncServiceClient.StreamingDataCaptureUpload(ctx, opts...))
	}
	return errtrace.Wrap2(client.StreamingDataCaptureUploadFunc(ctx, opts...))
}

// DataSyncServiceStreamingDataCaptureUploadClient represents a fake instance of
// a StreamingDataCaptureUpload client.
type DataSyncServiceStreamingDataCaptureUploadClient struct {
	datapb.DataSyncService_StreamingDataCaptureUploadClient
	SendFunc         func(*datapb.StreamingDataCaptureUploadRequest) error
	CloseAndRecvFunc func() (*datapb.StreamingDataCaptureUploadResponse, error)
}

// Send calls the injected Send or the real version.
func (client *DataSyncServiceStreamingDataCaptureUploadClient) Send(req *datapb.StreamingDataCaptureUploadRequest) error {
	if client.SendFunc == nil {
		return errtrace.Wrap(client.DataSyncService_StreamingDataCaptureUploadClient.Send(req))
	}
	return errtrace.Wrap(client.SendFunc(req))
}

// CloseAndRecv calls the injected CloseAndRecv or the real version.
func (client *DataSyncServiceStreamingDataCaptureUploadClient) CloseAndRecv() (*datapb.StreamingDataCaptureUploadResponse, error) {
	if client.CloseAndRecvFunc == nil {
		return errtrace.Wrap2(client.DataSyncService_StreamingDataCaptureUploadClient.CloseAndRecv())
	}
	return errtrace.Wrap2(client.CloseAndRecvFunc())
}

// DataSyncServiceFileUploadClient represents a fake instance of a FileUpload client.
type DataSyncServiceFileUploadClient struct {
	datapb.DataSyncService_FileUploadClient
	SendFunc         func(*datapb.FileUploadRequest) error
	CloseAndRecvFunc func() (*datapb.FileUploadResponse, error)
}

// Send calls the injected Send or the real version.
func (client *DataSyncServiceFileUploadClient) Send(req *datapb.FileUploadRequest) error {
	if client.SendFunc == nil {
		return errtrace.Wrap(client.DataSyncService_FileUploadClient.Send(req))
	}
	return errtrace.Wrap(client.SendFunc(req))
}

// CloseAndRecv calls the injected CloseAndRecv or the real version.
func (client *DataSyncServiceFileUploadClient) CloseAndRecv() (*datapb.FileUploadResponse, error) {
	if client.CloseAndRecvFunc == nil {
		return errtrace.Wrap2(client.DataSyncService_FileUploadClient.CloseAndRecv())
	}
	return errtrace.Wrap2(client.CloseAndRecvFunc())
}

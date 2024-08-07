package builtin

import (
	"context"

	v1 "go.viam.com/api/app/datasync/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func noOpCloudClientConstructor(grpc.ClientConnInterface) v1.DataSyncServiceClient {
	return &noOpCloudClient{}
}

type noOpCloudClient struct{}

func (*noOpCloudClient) DataCaptureUpload(
	ctx context.Context,
	in *v1.DataCaptureUploadRequest,
	opts ...grpc.CallOption,
) (*v1.DataCaptureUploadResponse, error) {
	return &v1.DataCaptureUploadResponse{}, nil
}
func (*noOpCloudClient) FileUpload(
	ctx context.Context,
	opts ...grpc.CallOption,
) (v1.DataSyncService_FileUploadClient, error) {
	return &noOpFileUploadClient{}, nil
}
func (*noOpCloudClient) StreamingDataCaptureUpload(
	ctx context.Context,
	opts ...grpc.CallOption,
) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
	return &noOpDataCaptureUploadClient{}, nil
}

type noOpFileUploadClient struct{ noOpClientStream }

func (*noOpFileUploadClient) Send(*v1.FileUploadRequest) error {
	return nil
}

func (*noOpFileUploadClient) CloseAndRecv() (*v1.FileUploadResponse, error) {
	return &v1.FileUploadResponse{}, nil
}

type noOpDataCaptureUploadClient struct{ noOpClientStream }

func (*noOpDataCaptureUploadClient) Send(*v1.StreamingDataCaptureUploadRequest) error {
	return nil
}

func (*noOpDataCaptureUploadClient) CloseAndRecv() (*v1.StreamingDataCaptureUploadResponse, error) {
	return &v1.StreamingDataCaptureUploadResponse{}, nil
}

type noOpClientStream struct{}

func (m *noOpClientStream) Header() (metadata.MD, error) {
	return metadata.MD{}, nil
}

func (m *noOpClientStream) Trailer() metadata.MD {
	return metadata.MD{}
}

func (m *noOpClientStream) CloseSend() error {
	return nil
}

func (m *noOpClientStream) Context() context.Context {
	return context.Background()
}

func (m *noOpClientStream) SendMsg(any) error {
	return nil
}

func (m *noOpClientStream) RecvMsg(any) error {
	return nil
}

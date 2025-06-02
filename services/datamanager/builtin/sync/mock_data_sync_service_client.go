package sync

import (
	"context"
	"errors"
	"testing"

	v1 "go.viam.com/api/app/datasync/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// MockDataSyncServiceClient is a mock for a v1.DataSyncServiceClient.
type MockDataSyncServiceClient struct {
	T                     *testing.T
	DataCaptureUploadFunc func(
		ctx context.Context,
		in *v1.DataCaptureUploadRequest,
		opts ...grpc.CallOption,
	) (*v1.DataCaptureUploadResponse, error)
	FileUploadFunc func(
		ctx context.Context,
		opts ...grpc.CallOption,
	) (v1.DataSyncService_FileUploadClient, error)
	StreamingDataCaptureUploadFunc func(
		ctx context.Context,
		opts ...grpc.CallOption,
	) (v1.DataSyncService_StreamingDataCaptureUploadClient, error)
}

// NoOpCloudClientConstructor returns a v1.DataSyncServiceClient that does nothing & always returns successes.
func NoOpCloudClientConstructor(grpc.ClientConnInterface) v1.DataSyncServiceClient {
	return &MockDataSyncServiceClient{
		DataCaptureUploadFunc: func(
			context.Context,
			*v1.DataCaptureUploadRequest,
			...grpc.CallOption,
		) (*v1.DataCaptureUploadResponse, error) {
			return &v1.DataCaptureUploadResponse{}, nil
		},
		StreamingDataCaptureUploadFunc: func(
			context.Context,
			...grpc.CallOption,
		) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
			return &ClientStreamingMock[
				*v1.StreamingDataCaptureUploadRequest,
				*v1.StreamingDataCaptureUploadResponse,
			]{
				SendFunc: func(*v1.StreamingDataCaptureUploadRequest) error {
					return nil
				},
				CloseAndRecvFunc: func() (*v1.StreamingDataCaptureUploadResponse, error) {
					return &v1.StreamingDataCaptureUploadResponse{}, nil
				},
			}, nil
		},
		FileUploadFunc: func(
			context.Context,
			...grpc.CallOption,
		) (v1.DataSyncService_FileUploadClient, error) {
			return &ClientStreamingMock[
				*v1.FileUploadRequest,
				*v1.FileUploadResponse,
			]{
				SendFunc: func(*v1.FileUploadRequest) error {
					return nil
				},
				CloseAndRecvFunc: func() (*v1.FileUploadResponse, error) {
					return &v1.FileUploadResponse{}, nil
				},
			}, nil
		},
	}
}

// DataCaptureUpload is needed to satisfy the interface.
func (c MockDataSyncServiceClient) DataCaptureUpload(
	ctx context.Context,
	in *v1.DataCaptureUploadRequest,
	opts ...grpc.CallOption,
) (*v1.DataCaptureUploadResponse, error) {
	if c.DataCaptureUploadFunc == nil {
		err := errors.New("DataCaptureUpload unimplemented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, err
	}
	return c.DataCaptureUploadFunc(ctx, in, opts...)
}

// FileUpload is needed to satisfy the interface.
func (c MockDataSyncServiceClient) FileUpload(
	ctx context.Context,
	opts ...grpc.CallOption,
) (v1.DataSyncService_FileUploadClient, error) {
	if c.FileUploadFunc == nil {
		err := errors.New("FileUpload unimplmented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, err
	}
	return c.FileUploadFunc(ctx, opts...)
}

// StreamingDataCaptureUpload is needed to satisfy the interface.
func (c MockDataSyncServiceClient) StreamingDataCaptureUpload(
	ctx context.Context,
	opts ...grpc.CallOption,
) (v1.DataSyncService_StreamingDataCaptureUploadClient, error) {
	if c.StreamingDataCaptureUploadFunc == nil {
		err := errors.New("StreamingDataCaptureUpload unimplmented")
		c.T.Log(err)
		c.T.FailNow()
		return nil, errors.New("StreamingDataCaptureUpload unimplmented")
	}
	return c.StreamingDataCaptureUploadFunc(ctx, opts...)
}

// ClientStreamingMock is a generic mock for a client streaming GRPC client.
type ClientStreamingMock[Req any, Res any] struct {
	NoOpClientStream
	T                *testing.T
	SendFunc         func(Req) error
	CloseAndRecvFunc func() (Res, error)
}

// Send implemnents Send for a generic mock for a client streaming GRPC client.
func (m *ClientStreamingMock[Req, Res]) Send(in Req) error {
	if m.SendFunc == nil {
		err := errors.New("Send unimplmented")
		m.T.Log(err)
		m.T.FailNow()
		return err
	}
	return m.SendFunc(in)
}

// CloseAndRecv implemnents CloseAndRecv for a generic mock for a client streaming GRPC client.
func (m *ClientStreamingMock[Req, Res]) CloseAndRecv() (Res, error) {
	if m.CloseAndRecvFunc == nil {
		err := errors.New("CloseAndRecv unimplmented")
		m.T.Log(err)
		m.T.FailNow()
		var zero Res
		return zero, err
	}
	return m.CloseAndRecvFunc()
}

// NoOpClientStream is a mock for a GRPC ClientStream that does nothing.
type NoOpClientStream struct{}

// Header satisfies the interface.
func (m *NoOpClientStream) Header() (metadata.MD, error) {
	return metadata.MD{}, nil
}

// Trailer satisfies the interface.
func (m *NoOpClientStream) Trailer() metadata.MD {
	return metadata.MD{}
}

// CloseSend satisfies the interface.
func (m *NoOpClientStream) CloseSend() error {
	return nil
}

// Context satisfies the interface.
func (m *NoOpClientStream) Context() context.Context {
	return context.Background()
}

// SendMsg satisfies the interface.
func (m *NoOpClientStream) SendMsg(any) error {
	return nil
}

// RecvMsg satisfies the interface.
func (m *NoOpClientStream) RecvMsg(any) error {
	return nil
}

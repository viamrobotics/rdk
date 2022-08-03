package datamanager

import (
	"context"
	"io"

	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"google.golang.org/grpc"
)

type mockDataSyncService_UploadServer struct {
	messagesSent               int
	sendAckEveryNMessages      int
	cancelStreamAfterNMessages int
	shouldSendEOF              bool
	shouldSendACK              bool
	shouldSendCancelCtx        bool
	grpc.ServerStream
}

func (m *mockDataSyncService_UploadServer) Send(ur *v1.UploadResponse) error {
	m.messagesSent++
	if m.messagesSent == m.sendAckEveryNMessages {
		m.messagesSent = 0
		m.shouldSendACK = true
	} else {
		if m.messagesSent == m.cancelStreamAfterNMessages {
			m.shouldSendCancelCtx = true
		}
	}
	return m.ServerStream.SendMsg(ur)

}
func (m *mockDataSyncService_UploadServer) Recv() (*v1.UploadRequest, error) {
	ur := new(v1.UploadRequest)
	if err := m.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return ur, nil
}

func uploadReqToUploadResp(m *mockDataSyncService_UploadServer, req *v1.UploadRequest) (*v1.UploadResponse, error) {
	if m.shouldSendACK {
		m.shouldSendACK = false
		return &v1.UploadResponse{RequestsWritten: int32(m.messagesSent)}, nil
	}
	if m.shouldSendEOF {
		m.shouldSendEOF = false
		return nil, io.EOF
	}
	if m.shouldSendCancelCtx {
		m.shouldSendCancelCtx = false
		return nil, context.Canceled
	}
	return &v1.UploadResponse{}, nil
}

type mockDataSyncServiceServer struct {
	v1.UnimplementedDataSyncServiceServer
}

func (m mockDataSyncServiceServer) Upload(stream v1.DataSyncService_UploadServer) error {
	mockServer := &mockDataSyncService_UploadServer{
		messagesSent:               0,
		sendAckEveryNMessages:      3,
		cancelStreamAfterNMessages: -1,
		shouldSendEOF:              false,
		shouldSendACK:              false,
	}
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		res, err := uploadReqToUploadResp(mockServer, req)
		if err != nil {
			return err
		}
		stream.Send(res)
	}
	return nil
}

//nolint: unused
func (m mockDataSyncServiceServer) mustEmbedUnimplementedDataSyncServiceServer() {}

package datamanager

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/edaniels/golog"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
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

func TestDataCaptureUpload(t *testing.T) {
	// Register mock datamanager service with a mock server.
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	rpcServer.RegisterServiceServer(
		context.Background(),
		&v1.DataSyncService_ServiceDesc,
		mockDataSyncServiceServer{},
		v1.RegisterDataSyncServiceHandlerFromEndpoint,
	)

	// Stand up the server. Defer stopping the server.
	go func() {
		err := rpcServer.Start()
		test.That(t, err, test.ShouldBeNil)
	}()
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	// Dial connection.
	port, err := utils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	rawAddress := fmt.Sprintf("localhost:%d", port)
	test.That(t, err, test.ShouldBeNil)
	conn, err := rpc.DialDirectGRPC(
		context.Background(),
		rawAddress,
		logger,
	)
	test.That(t, err, test.ShouldBeNil)

	// Defer closing the connection.
	defer func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()
	test.That(t, err, test.ShouldBeNil)
	client := v1.NewDataSyncServiceClient(conn)
	print(client)

	// Validate that the client responds properly to whatever is sent by the mocked server.

}

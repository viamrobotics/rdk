package datamanager

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
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
	messagesSent               int
	sendAckEveryNMessages      int
	cancelStreamAfterNMessages int
	shouldSendEOF              bool
	shouldSendACK              bool
	shouldSendCancelCtx        bool
	v1.UnimplementedDataSyncServiceServer
}

func (m mockDataSyncServiceServer) Upload(stream v1.DataSyncService_UploadServer) error {
	mockServer := &mockDataSyncService_UploadServer{
		messagesSent:               m.messagesSent,
		sendAckEveryNMessages:      m.sendAckEveryNMessages,
		cancelStreamAfterNMessages: m.cancelStreamAfterNMessages,
		shouldSendEOF:              m.shouldSendEOF,
		shouldSendACK:              m.shouldSendACK,
		shouldSendCancelCtx:        m.shouldSendCancelCtx,
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

type mockServerBehavior struct {
	sendAckEveryNMessages      int
	cancelStreamAfterNMessages int
	shouldSendEOF              bool
	shouldSendAck              bool
	shouldSendCancelCtx        bool
}

func TestDataCaptureUpload(t *testing.T) {
	msgBin1 := []byte("Robots are really cool.")
	msgBin2 := []byte("This work is helping develop the robotics space.")
	msgBin3 := []byte("This message is used for testing.")
	tests := []struct {
		name    string
		toSend  [][]byte
		expData [][]byte
		msb     *mockServerBehavior
	}{
		{
			name:    "stream of binary sensor data readings",
			toSend:  [][]byte{msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3},
			expData: [][]byte{msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3},
			msb: &mockServerBehavior{
				sendAckEveryNMessages:      2,
				cancelStreamAfterNMessages: -1,
				shouldSendEOF:              false,
				shouldSendAck:              false,
				shouldSendCancelCtx:        false,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mc := &mockDataSyncServiceServer{
				messagesSent:               0,
				sendAckEveryNMessages:      tc.msb.sendAckEveryNMessages,
				cancelStreamAfterNMessages: tc.msb.cancelStreamAfterNMessages,
				shouldSendEOF:              tc.msb.shouldSendEOF,
				shouldSendACK:              tc.msb.shouldSendAck,
				shouldSendCancelCtx:        tc.msb.shouldSendCancelCtx,
			}

			// Register mock datamanager service with a mock server.
			logger, _ := golog.NewObservedTestLogger(t)
			rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
			test.That(t, err, test.ShouldBeNil)
			rpcServer.RegisterServiceServer(
				context.Background(),
				&v1.DataSyncService_ServiceDesc,
				mc,
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
				err := conn.Close()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Create temp file to be used as examples of reading data from the files into buffers and finally to have
			// that data be uploaded to the cloud
			tf, err := createTmpDataCaptureFile()
			if err != nil {
				t.Errorf("%s cannot create temporary file to be used for sensorUpload/fileUpload testing: %v",
					tc.name, err)
			}
			defer os.Remove(tf.Name())

			// First write metadata to file.
			syncMetadata := v1.DataCaptureMetadata{
				ComponentType:    componentType,
				ComponentName:    componentName,
				MethodName:       methodName,
				Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
				MethodParameters: nil,
			}
			if _, err := pbutil.WriteDelimited(tf, &syncMetadata); err != nil {
				t.Errorf("%s cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v",
					tc.name, err)
			}

			// Write the data from the test cases into the files to prepare them for reading by the sensorUpload function.
			if err := writeBinarySensorData(tf, tc.toSend); err != nil {
				t.Errorf("%s cannot write byte slice to temporary file as part of setup for "+
					"sensorUpload/fileUpload testing: %v", tc.name, err)
			}

			// Validate that the client responds properly to whatever is sent by the mocked server.
			client := v1.NewDataSyncServiceClient(conn)
			_, err = client.Upload(context.Background())
			test.That(t, err, test.ShouldBeNil)

		})
	}

}

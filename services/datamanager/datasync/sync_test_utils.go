package datasync

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/services/datamanager/datacapture"
)

var (
	partID        = "partid"
	componentType = "componenttype"
	componentName = "componentname"
	methodName    = "methodname"
)

// Compares UploadRequests containing either binary or tabular sensor data.
// nolint:thelper
func compareUploadRequests(t *testing.T, isTabular bool, actual []*v1.UploadRequest, expected []*v1.UploadRequest) {
	// Ensure length of slices is same before proceeding with rest of tests.
	test.That(t, len(actual), test.ShouldEqual, len(expected))

	if len(actual) > 0 {
		// Compare metadata upload requests (uncomment below).
		compareMetadata(t, actual[0].GetMetadata(), expected[0].GetMetadata())

		// Compare data differently for binary & tabular data.
		if isTabular {
			// Compare tabular data upload request (stream).
			for i, uploadRequest := range actual[1:] {
				a := uploadRequest.GetSensorContents().GetStruct()
				e := actual[i+1].GetSensorContents().GetStruct()
				test.That(t, a, test.ShouldResemble, e)
			}
		} else {
			// Compare sensor data upload request (stream).
			for i, uploadRequest := range actual[1:] {
				a := uploadRequest.GetSensorContents().GetBinary()
				e := expected[i+1].GetSensorContents().GetBinary()
				test.That(t, a, test.ShouldResemble, e)
			}
		}
	}
}

// nolint:thelper
func compareMetadata(t *testing.T, actualMetadata *v1.UploadMetadata,
	expectedMetadata *v1.UploadMetadata,
) {
	// Test the fields within UploadRequest Metadata.
	test.That(t, filepath.Base(actualMetadata.FileName), test.ShouldEqual, filepath.Base(expectedMetadata.FileName))
	test.That(t, actualMetadata.PartId, test.ShouldEqual, expectedMetadata.PartId)
	test.That(t, actualMetadata.ComponentName, test.ShouldEqual, expectedMetadata.ComponentName)
	test.That(t, actualMetadata.ComponentType, test.ShouldEqual, expectedMetadata.ComponentType)
	test.That(t, actualMetadata.MethodName, test.ShouldEqual, expectedMetadata.MethodName)
	test.That(t, actualMetadata.Type, test.ShouldEqual, expectedMetadata.Type)
}

type anyStruct struct {
	Field1 bool
	Field2 int
	Field3 string
}

func toProto(r interface{}) *structpb.Struct {
	msg, err := protoutils.StructToStructPb(r)
	if err != nil {
		return nil
	}
	return msg
}

func writeBinarySensorData(f *os.File, toWrite [][]byte) error {
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	for _, bytes := range toWrite {
		sd := &v1.SensorData{
			Data: &v1.SensorData_Binary{
				Binary: bytes,
			},
		}
		_, err := pbutil.WriteDelimited(f, sd)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeSensorData(f *os.File, sds []*v1.SensorData) error {
	for _, sd := range sds {
		_, err := pbutil.WriteDelimited(f, sd)
		if err != nil {
			return err
		}
	}
	return nil
}

func createBinarySensorData(toWrite [][]byte) []*v1.SensorData {
	var sds []*v1.SensorData
	for _, bytes := range toWrite {
		sd := &v1.SensorData{
			Data: &v1.SensorData_Binary{
				Binary: bytes,
			},
		}
		sds = append(sds, sd)
	}
	return sds
}

func createTabularSensorData(toWrite []*structpb.Struct) []*v1.SensorData {
	var sds []*v1.SensorData
	for _, contents := range toWrite {
		sd := &v1.SensorData{
			Data: &v1.SensorData_Struct{
				Struct: contents,
			},
		}
		sds = append(sds, sd)
	}
	return sds
}

// createTmpDataCaptureFile creates a data capture file, which is defined as a file with the dataCaptureFileExt as its
// file extension.
func createTmpDataCaptureFile() (file *os.File, err error) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	if err = os.Rename(tf.Name(), tf.Name()+datacapture.FileExt); err != nil {
		return nil, err
	}
	ret, err := os.OpenFile(tf.Name()+datacapture.FileExt, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func buildBinaryUploadRequests(data [][]byte, fileName string) []*v1.UploadRequest {
	var expMsgs []*v1.UploadRequest
	if len(data) == 0 {
		return expMsgs
	}
	// Metadata message precedes sensor data messages.
	expMsgs = append(expMsgs, &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:        partID,
				Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
				FileName:      fileName,
				ComponentType: componentType,
				ComponentName: componentName,
				MethodName:    methodName,
			},
		},
	})
	for _, expMsg := range data {
		expMsgs = append(expMsgs, &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_SensorContents{
				SensorContents: &v1.SensorData{
					Data: &v1.SensorData_Binary{
						Binary: expMsg,
					},
				},
			},
		})
	}
	return expMsgs
}

func getMockService() mockDataSyncServiceServer {
	return mockDataSyncServiceServer{
		uploadRequests:                     &[]*v1.UploadRequest{},
		callCount:                          &atomic.Int32{},
		failAtIndex:                        -1,
		lock:                               &sync.Mutex{},
		UnimplementedDataSyncServiceServer: v1.UnimplementedDataSyncServiceServer{},

		// Fields below this line added by maxhorowitz
		sendAckEveryNMessages:       3,
		reqsStagedForResponse:       0,
		sendCancelCtxAfterNMessages: -1,
	}
}

//nolint:thelper
func buildAndStartLocalServer(t *testing.T, logger golog.Logger, mockService mockDataSyncServiceServer) rpc.Server {
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	err = rpcServer.RegisterServiceServer(
		context.Background(),
		&v1.DataSyncService_ServiceDesc,
		mockService,
		v1.RegisterDataSyncServiceHandlerFromEndpoint,
	)
	test.That(t, err, test.ShouldBeNil)

	// Stand up the server. Defer stopping the server.
	go func() {
		err := rpcServer.Start()
		test.That(t, err, test.ShouldBeNil)
	}()
	return rpcServer
}

func getLocalServerConn(rpcServer rpc.Server, logger golog.Logger) (rpc.ClientConn, error) {
	return rpc.DialDirectGRPC(
		context.Background(),
		rpcServer.InternalAddr().String(),
		logger,
		rpc.WithInsecure(),
	)
}

type mockDataSyncServiceServer struct {
	uploadRequests *[]*v1.UploadRequest
	callCount      *atomic.Int32
	failUntilIndex int32
	failAtIndex    int32

	lock *sync.Mutex
	v1.UnimplementedDataSyncServiceServer

	// Fields below this line added by maxhorowitz
	sendAckEveryNMessages       int
	reqsStagedForResponse       int
	sendCancelCtxAfterNMessages int
}

func (m mockDataSyncServiceServer) getUploadRequests() []*v1.UploadRequest {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	return *m.uploadRequests
}

func (m mockDataSyncServiceServer) Upload(stream v1.DataSyncService_UploadServer) error {
	defer m.callCount.Add(1)
	if m.callCount.Load() < m.failUntilIndex {
		return status.Error(codes.Aborted, "fail until reach failUntilIndex")
	}

	m.reqsStagedForResponse = 0
	for {
		// If server.Upload(stream) has been called too many times, abort.
		if m.callCount.Load() == m.failAtIndex {
			return status.Error(codes.Aborted, "failed at failAtIndex")
		}

		// Keeps track of total requests processed.
		reqsProcessed := len(m.getUploadRequests())

		// If we should cancel ctx, cancel.
		if m.sendCancelCtxAfterNMessages == reqsProcessed {
			if err := stream.Send(&v1.UploadResponse{RequestsWritten: int32(m.reqsStagedForResponse)}); err != nil {
				return err
			}
			return context.Canceled
		}

		// Recv UploadRequest (block until received).
		ur, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		// Append UploadRequest to list of recorded requests.
		(*m.lock).Lock()
		newData := append(*m.uploadRequests, ur)
		*m.uploadRequests = newData
		m.reqsStagedForResponse++
		(*m.lock).Unlock()

		// Send an ACK at intervals of N messages.
		if m.reqsStagedForResponse == m.sendAckEveryNMessages {
			if err := stream.Send(&v1.UploadResponse{RequestsWritten: int32(m.reqsStagedForResponse)}); err != nil {
				return err
			}
			m.reqsStagedForResponse = 0
		}
	}
	return nil
}

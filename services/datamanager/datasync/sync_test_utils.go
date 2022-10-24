package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

var (
	sendWaitTime   = time.Millisecond * 25
	partID         = "partid"
	componentType  = "componenttype"
	componentName  = "componentname"
	componentModel = "componentmodel"
	tags           = []string{"tagA", "tagB"}
	methodName     = "NextPointCloud"
)

// Compares UploadRequests containing either binary or tabular sensor data.
func compareTabularUploadRequests(t *testing.T, actual, expected []*v1.UploadRequest) {
	t.Helper()
	// Ensure length of slices is same before proceeding with rest of tests.
	test.That(t, len(actual), test.ShouldEqual, len(expected))

	if len(actual) > 0 {
		test.That(t, actual[0].GetMetadata().String(), test.ShouldResemble, expected[0].GetMetadata().String())
		for i := range actual[1:] {
			a := actual[i+1].GetSensorContents().GetStruct().String()
			e := expected[i+1].GetSensorContents().GetStruct().String()
			test.That(t, a, test.ShouldResemble, e)
		}
	}
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

func buildSensorDataUploadRequests(sds []*v1.SensorData, dataType v1.DataType, filePath string) []*v1.UploadRequest {
	var expMsgs []*v1.UploadRequest
	if len(sds) == 0 {
		return expMsgs
	}
	// Metadata message precedes sensor data messages.
	expMsgs = append(expMsgs, &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:         partID,
				Type:           dataType,
				FileName:       filepath.Base(filePath),
				ComponentType:  componentType,
				ComponentName:  componentName,
				ComponentModel: componentModel,
				MethodName:     methodName,
				FileExtension:  datacapture.GetFileExt(dataType, methodName, nil),
				Tags:           tags,
			},
		},
	})
	for _, expMsg := range sds {
		expMsgs = append(expMsgs, &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_SensorContents{
				SensorContents: expMsg,
			},
		})
	}
	return expMsgs
}

func buildFileDataUploadRequests(bs [][]byte, fileName string) []*v1.UploadRequest {
	var expMsgs []*v1.UploadRequest
	// Metadata message precedes sensor data messages.
	expMsgs = append(expMsgs, &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:   partID,
				Type:     v1.DataType_DATA_TYPE_FILE,
				FileName: fileName,
			},
		},
	})
	for _, b := range bs {
		expMsgs = append(expMsgs, &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_FileContents{
				FileContents: &v1.FileData{Data: b},
			},
		})
	}
	return expMsgs
}

// createTmpDataCaptureFile creates a data capture file, which is defined as a file with the dataCaptureFileExt as its
// file extension.
func createTmpDataCaptureFile() (file *os.File, err error) {
	tf, err := os.CreateTemp("", "")
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

//nolint:thelper
func buildAndStartLocalServer(t *testing.T, logger golog.Logger, mockService *mockDataSyncServiceServer) rpc.Server {
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	err = rpcServer.RegisterServiceServer(
		context.Background(),
		&v1.DataSyncService_ServiceDesc,
		// TODO: why does this break on partial uploads without dereference?
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
	errorToReturn  error

	lock *sync.Mutex
	v1.UnimplementedDataSyncServiceServer

	messagesPerAck      int
	messagesToAck       int
	clientShutdownIndex int
	cancelChannel       chan bool
	doneCancelChannel   chan bool
}

func getMockService() *mockDataSyncServiceServer {
	return &mockDataSyncServiceServer{
		uploadRequests:                     &[]*v1.UploadRequest{},
		callCount:                          &atomic.Int32{},
		failAtIndex:                        -1,
		lock:                               &sync.Mutex{},
		UnimplementedDataSyncServiceServer: v1.UnimplementedDataSyncServiceServer{},
		errorToReturn:                      errors.New("generic error goes here"),
		messagesPerAck:                     1,
		messagesToAck:                      0,
		clientShutdownIndex:                -1,
	}
}

func (m mockDataSyncServiceServer) getUploadRequests() []*v1.UploadRequest {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	return *m.uploadRequests
}

func (m mockDataSyncServiceServer) Upload(stream v1.DataSyncService_UploadServer) error {
	defer m.callCount.Add(1)
	if m.callCount.Load() < m.failUntilIndex {
		return m.errorToReturn
	}
	m.messagesToAck = 0
	for {
		if len(m.getUploadRequests()) == int(m.failAtIndex) {
			return m.errorToReturn
		}

		ur, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		// Append UploadRequest to list of recorded requests.
		(*m.lock).Lock()
		*m.uploadRequests = append(*m.uploadRequests, ur)
		if ur.GetMetadata() == nil {
			m.messagesToAck++
		}
		(*m.lock).Unlock()

		if m.messagesToAck == m.messagesPerAck {
			if err := stream.Send(&v1.UploadResponse{RequestsWritten: int32(m.messagesToAck)}); err != nil {
				return err
			}
			time.Sleep(sendWaitTime)
			m.messagesToAck = 0
		}

		// If we want the client to cancel its own context, send signal through channel to the client, then wait for
		// client to Close. This simulates a client's context being cancelled before receiving a sent ACK.
		if m.clientShutdownIndex == len(m.getUploadRequests())-1 {
			m.cancelChannel <- true
			<-m.doneCancelChannel
			break
		}
	}
	return nil
}

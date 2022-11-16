package datasync

import (
	"context"
	"go.viam.com/utils/protoutils"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
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

//func buildSensorDataUploadRequests(sds []*v1.SensorData, dataType v1.DataType, filePath string) []*v1.UploadRequest {
//	var expMsgs []*v1.UploadRequest
//	if len(sds) == 0 {
//		return expMsgs
//	}
//	// Metadata message precedes sensor data messages.
//	expMsgs = append(expMsgs, &v1.UploadRequest{
//		UploadPacket: &v1.UploadRequest_Metadata{
//			Metadata: &v1.UploadMetadata{
//				PartId:         partID,
//				Type:           dataType,
//				FileName:       filepath.Base(filePath),
//				ComponentType:  componentType,
//				ComponentName:  componentName,
//				ComponentModel: componentModel,
//				MethodName:     methodName,
//				FileExtension:  datacapture.GetFileExt(dataType, methodName, nil),
//				Tags:           tags,
//			},
//		},
//	})
//	for _, expMsg := range sds {
//		expMsgs = append(expMsgs, &v1.UploadRequest{
//			UploadPacket: &v1.UploadRequest_SensorContents{
//				SensorContents: expMsg,
//			},
//		})
//	}
//	return expMsgs
//}

//func buildFileDataUploadRequests(bs [][]byte, fileName string) []*v1.UploadRequest {
//	var expMsgs []*v1.UploadRequest
//	// Metadata message precedes sensor data messages.
//	expMsgs = append(expMsgs, &v1.UploadRequest{
//		UploadPacket: &v1.UploadRequest_Metadata{
//			Metadata: &v1.UploadMetadata{
//				PartId:   partID,
//				Type:     v1.DataType_DATA_TYPE_FILE,
//				FileName: fileName,
//			},
//		},
//	})
//	for _, b := range bs {
//		expMsgs = append(expMsgs, &v1.UploadRequest{
//			UploadPacket: &v1.UploadRequest_FileContents{
//				FileContents: &v1.FileData{Data: b},
//			},
//		})
//	}
//	return expMsgs
//}

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
	uploadRequests []*v1.DataCaptureUploadRequest
	callCount      *atomic.Int32
	failUntilIndex int32
	failAtIndex    int32
	errorToReturn  error

	lock *sync.Mutex
	v1.UnimplementedDataSyncServiceServer
}

func getMockService() *mockDataSyncServiceServer {
	return &mockDataSyncServiceServer{
		uploadRequests:                     []*v1.DataCaptureUploadRequest{},
		callCount:                          &atomic.Int32{},
		failAtIndex:                        -1,
		lock:                               &sync.Mutex{},
		UnimplementedDataSyncServiceServer: v1.UnimplementedDataSyncServiceServer{},
		errorToReturn:                      errors.New("generic error goes here"),
	}
}

func (m mockDataSyncServiceServer) getUploadRequests() []*v1.DataCaptureUploadRequest {
	(*m.lock).Lock()
	defer (*m.lock).Unlock()
	return m.uploadRequests
}

func (m mockDataSyncServiceServer) DataCaptureUpload(ctx context.Context, ur *v1.DataCaptureUploadRequest) (*v1.DataCaptureUploadResponse, error) {
	defer m.callCount.Add(1)
	if m.callCount.Load() < m.failUntilIndex {
		return nil, m.errorToReturn
	}

	if len(m.getUploadRequests()) == int(m.failAtIndex) {
		return nil, m.errorToReturn
	}

	// Append UploadRequest to list of recorded requests.
	(*m.lock).Lock()
	m.uploadRequests = append(m.uploadRequests, ur)
	(*m.lock).Unlock()

	return &v1.DataCaptureUploadResponse{
		Code:    200,
		Message: "yay",
	}, nil
}

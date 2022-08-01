package datamanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
)

var (
	partID        = "partid"
	componentType = "componenttype"
	componentName = "componentname"
	methodName    = "methodname"
)

// mockClient implements DataSyncService_UploadClient and maintains a list of all UploadRequests sent with its
// send method. The mockClient shuts down after a maximum of 'cancelIndex+1' sent UploadRequests. The '+1' gives
// capacity for the metadata message to precede other messages. This simulates partial uploads (cases where client is
// shut down during upload).
type mockClient struct {
	sent        []*v1.UploadRequest
	cancelIndex int
	lock        sync.Mutex
	grpc.ClientStream
}

func (m *mockClient) Send(req *v1.UploadRequest) error {
	m.lock.Lock()
	if m.cancelIndex != len(m.sent) {
		m.sent = append(m.sent, req)
		m.lock.Unlock()
		return nil
	}
	m.lock.Unlock()
	return errors.New("cancel sending of upload request")
}

func (m *mockClient) CloseAndRecv() (*v1.UploadResponse, error) {
	return &v1.UploadResponse{}, nil
}

func (m *mockClient) Context() context.Context {
	return context.TODO()
}

// Builds syncer used in partial upload tests.
//nolint:thelper
func newTestSyncer(t *testing.T, mc *mockClient, uploadFunc uploadFunc) *syncer {
	l := golog.NewTestLogger(t)
	ret, err := newSyncer(l, uploadFunc, partID)
	test.That(t, err, test.ShouldBeNil)
	ret.client = mc
	return ret
}

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
	sds := []*v1.SensorData{}
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
	sds := []*v1.SensorData{}
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

func getUploadRequests(sds []*v1.SensorData, dt v1.DataType, fileName string) []*v1.UploadRequest {
	urs := []*v1.UploadRequest{}
	if len(sds) == 0 {
		return []*v1.UploadRequest{}
	}
	urs = append(urs, &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:        partID,
				Type:          dt,
				FileName:      fileName,
				ComponentType: componentType,
				ComponentName: componentName,
				MethodName:    methodName,
			},
		},
	})
	for _, sd := range sds {
		urs = append(urs, &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_SensorContents{SensorContents: sd},
		})
	}
	return urs
}

// createTmpDataCaptureFile creates a data capture file, which is defined as a file with the dataCaptureFileExt as its
// file extension.
func createTmpDataCaptureFile() (file *os.File, err error) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	if err = os.Rename(tf.Name(), tf.Name()+dataCaptureFileExt); err != nil {
		return nil, err
	}
	ret, err := os.OpenFile(tf.Name()+dataCaptureFileExt, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func fileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return !errors.Is(err, os.ErrNotExist)
}

func buildBinarySensorMsgs(data [][]byte, fileName string) []*v1.UploadRequest {
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

type partialUploadTestcase struct {
	name                      string
	toSend                    []*v1.SensorData
	progressIndexWhenCanceled int
	expDataBeforeCanceled     []*v1.SensorData
	expDataAfterCanceled      []*v1.SensorData
	dataType                  v1.DataType
}

func initMockClient(lenMsgsToSend int) *mockClient {
	// cancelIndex gives mock client capacity to "send" metadata message in addition to succeeding sensordata
	// messages.
	cancelIndex := 0
	if lenMsgsToSend != 0 {
		cancelIndex = lenMsgsToSend + 1
	}
	return &mockClient{
		sent:        []*v1.UploadRequest{},
		lock:        sync.Mutex{},
		cancelIndex: cancelIndex,
	}
}

// nolint:thelper
func writeCaptureMetadataToFile(t *testing.T, dt v1.DataType, tf *os.File) {
	// First write metadata to file.
	captureMetadata := v1.DataCaptureMetadata{
		ComponentType:    componentType,
		ComponentName:    componentName,
		MethodName:       methodName,
		Type:             dt,
		MethodParameters: nil,
	}
	if _, err := pbutil.WriteDelimited(tf, &captureMetadata); err != nil {
		t.Errorf("cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v", err)
	}
}

// nolint:thelper
func compareUploadRequestsMockClient(t *testing.T, isTabular bool, mc *mockClient, expMsgs []*v1.UploadRequest) {
	mc.lock.Lock()
	compareUploadRequests(t, false, mc.sent, expMsgs)
	mc.lock.Unlock()
}

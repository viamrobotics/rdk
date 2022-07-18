package datamanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
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

// mockClient implements DataSyncService_UploadClient and maintains a list of all UploadRequests sent with its send
// method.
type mockClient struct {
	sent []*v1.UploadRequest
	lock sync.Mutex
	grpc.ClientStream
}

func (m *mockClient) Send(req *v1.UploadRequest) error {
	m.lock.Lock()
	m.sent = append(m.sent, req)
	m.lock.Unlock()
	return nil
}

func (m *mockClient) CloseAndRecv() (*v1.UploadResponse, error) {
	return &v1.UploadResponse{}, nil
}

func (m *mockClient) Context() context.Context {
	return context.TODO()
}

// Builds syncer used in tests.
func newTestSyncer(t *testing.T, mc *mockClient, uploadFn uploadFn) *syncer {
	t.Helper()
	l := golog.NewTestLogger(t)

	ret := *newSyncer(l, uploadFn, partID)
	ret.client = mc
	return &ret
}

// mockClientShutdown implements DataSyncService_UploadClient and maintains a list of all UploadRequests sent with its
// send method. It differs from mockClient because it shuts down after a maximum of three sent UploadRequests. This
// simulates partial uploads (cases where client loses connection from server during upload).
type mockClientShutdown struct {
	sent []*v1.UploadRequest
	lock sync.Mutex
	grpc.ClientStream
}

func (m *mockClientShutdown) Send(req *v1.UploadRequest) error {
	m.lock.Lock()
	// Shut down client after sending two sensordata messages -- those messages are always preceded by one metadata
	// message. For this reason, return an error when the client attempts to send its fourth message.
	if len(m.sent) < 3 {
		m.sent = append(m.sent, req)
		m.lock.Unlock()
		return nil
	}
	m.lock.Unlock()
	return errors.New("cannot mock client shutdown")
}

func (m *mockClientShutdown) CloseAndRecv() (*v1.UploadResponse, error) {
	return &v1.UploadResponse{}, nil
}

func (m *mockClientShutdown) Context() context.Context {
	return context.TODO()
}

// Builds syncer used in partial upload tests.
func newTestSyncerPartialUploads(t *testing.T, mc *mockClientShutdown, uploadFn uploadFn) *syncer {
	t.Helper()
	l := golog.NewTestLogger(t)
	ret := *newSyncer(l, uploadFn, partID)
	ret.client = mc
	return &ret
}

// Compares UploadRequests containing either binary or tabular sensor data.
// nolint
func compareUploadRequests(t *testing.T, isTabular bool, actual []*v1.UploadRequest, expected []*v1.UploadRequest) {
	// Ensure length of slices is same before proceeding with rest of tests.
	test.That(t, len(actual), test.ShouldEqual, len(expected))

	// Compare metadata upload requests (uncomment below).
	// compareMetadata(t, actual[0].GetMetadata(), expected[0].GetMetadata())

	if len(actual) > 0 {
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

// nolint
func compareMetadata(t *testing.T, actualMetadata *v1.UploadMetadata,
	expectedMetadata *v1.UploadMetadata,
) {
	// Test the fields within UploadRequest Metadata.
	test.That(t, filepath.Clean(actualMetadata.FileName), test.ShouldEqual, filepath.Clean(expectedMetadata.FileName))
	test.That(t, actualMetadata.PartId, test.ShouldEqual, expectedMetadata.PartId)
	test.That(t, actualMetadata.ComponentName, test.ShouldEqual, expectedMetadata.ComponentName)
	test.That(t, actualMetadata.ComponentType, test.ShouldEqual, expectedMetadata.ComponentType)
	test.That(t, actualMetadata.MethodName, test.ShouldEqual, expectedMetadata.MethodName)
	test.That(t, actualMetadata.Type, test.ShouldEqual, expectedMetadata.Type)
}

type anyStruct struct {
	Field bool
}

func toProto(r interface{}) *structpb.Struct {
	msg, err := protoutils.StructToStructPb(r)
	if err != nil {
		return nil
	}
	return msg
}

// Writes the protobuf message to the file passed into method. Returns the number of bytes written and any errors that
// are raised.
func writeBinarySensorData(f *os.File, toWrite [][]byte) error {
	for _, bytes := range toWrite {
		msg := &v1.SensorData{
			Data: &v1.SensorData_Binary{
				Binary: bytes,
			},
		}
		_, err := pbutil.WriteDelimited(f, msg)
		if err != nil {
			return err
		}
	}
	return nil
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

func createProgressFile(progress int, dcFileName string) (string, error) {
	path := filepath.Join(progressDir, filepath.Base(dcFileName))
	err := ioutil.WriteFile(path, intToBytes(progress), os.FileMode((0o777)))
	if err != nil {
		return "", err
	}
	return path, nil
}

func getProgressFromProgressFile(progressFileName string) (int, error) {
	bs, err := ioutil.ReadFile(filepath.Clean(progressFileName))
	if err != nil {
		return 0, err
	}
	i, err := bytesToInt(bs)
	if err != nil {
		return 0, err
	}
	return i, nil
}

// nolint
func verifyProgressFileContent(t *testing.T, progressAtBreakpoint int, progressFileName string) {
	//nolint
	progress, _ := getProgressFromProgressFile(progressFileName)
	test.That(t, reflect.DeepEqual(progressAtBreakpoint, progress), test.ShouldBeTrue)
}

// nolint
func verifyFileExistence(t *testing.T, fileName string, shouldExist bool) {
	_, err := os.Stat(fileName)
	test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldNotEqual, shouldExist)
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

func resetFolderContents(path string) error {
	fileInfo, err := os.Stat(path)
	if os.IsNotExist(err) {
		if err = os.Mkdir(path, os.ModePerm); err != nil {
			return err
		}
	} else {
		if fileInfo.IsDir() {
			infos, err := ioutil.ReadDir(path)
			if err != nil {
				return err
			}
			for _, info := range infos {
				if err = os.Remove(info.Name()); err != nil {
					return err
				}
			}
		} else {
			if err = os.Remove(fileInfo.Name()); err != nil {
				return err
			}
			return resetFolderContents(path)
		}
	}
	return nil
}

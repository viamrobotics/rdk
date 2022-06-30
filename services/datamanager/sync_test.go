package datamanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

// mockClient implements DataSyncService_UploadClient and maintains a list of all UploadRequests sent with its Send
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

// Compares UploadRequests (which hold either binary or tabular data).
func compareUploadRequests(t *testing.T, isTabular bool, actual []*v1.UploadRequest, expected []*v1.UploadRequest) {
	t.Helper()

	// Ensure length of slices is same before proceeding with rest of tests.
	test.That(t, len(actual), test.ShouldEqual, len(expected))

	// Compare metadata upload requests.
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

func compareMetadata(t *testing.T, actualMetadata *v1.UploadMetadata,
	expectedMetadata *v1.UploadMetadata,
) {
	t.Helper()

	// Test the fields within UploadRequest Metadata.
	test.That(t, actualMetadata.FileName, test.ShouldEqual, expectedMetadata.FileName)
	test.That(t, actualMetadata.PartId, test.ShouldEqual, expectedMetadata.PartId)
	test.That(t, actualMetadata.ComponentName, test.ShouldEqual, expectedMetadata.ComponentName)
	test.That(t, actualMetadata.MethodName, test.ShouldEqual, expectedMetadata.MethodName)
	test.That(t, actualMetadata.Type, test.ShouldEqual, expectedMetadata.Type)
}

// Builds syncer used in tests.
func newTestSyncer(t *testing.T, mc *mockClient, uploadFn uploadFn) *syncer {
	t.Helper()
	l := golog.NewTestLogger(t)

	ret := *newSyncer(l, uploadFn, partID)
	ret.client = mc
	return &ret
}

func TestFileUpload(t *testing.T) {
	uploadChunkSize = 10
	msgEmpty := []byte("")
	msgContents := []byte("This is part of testing in datamanager service in RDK.")

	tests := []struct {
		name    string
		toSend  []byte
		expData [][]byte
	}{
		{
			name:    "empty",
			toSend:  msgEmpty,
			expData: [][]byte{},
		},
		{
			name:   "not empty",
			toSend: msgContents,
			expData: [][]byte{
				msgContents[:10], msgContents[10:20], msgContents[20:30], msgContents[30:40],
				msgContents[40:50], msgContents[50:],
			},
		},
	}

	for _, tc := range tests {
		mc := &mockClient{
			sent: []*v1.UploadRequest{},
			lock: sync.Mutex{},
		}

		// Create temp file to be used as examples of reading data from the files into buffers
		// (and finally to have that data be uploaded) to the cloud.
		tf, err := ioutil.TempFile("", "")
		if err != nil {
			t.Errorf("%s: cannot create temporary file to be used for sensorUpload/fileUpload testing: %v", tc.name, err)
		}
		defer os.Remove(tf.Name())

		// Write the data from test cases into the temp file to prepare for reading by the fileUpload function.
		if _, err := tf.Write(tc.toSend); err != nil {
			t.Errorf("%s: cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing: %v", tc.name, err)
		}

		sut := newTestSyncer(t, mc, nil)
		sut.Sync([]string{tf.Name()})

		// Create []v1.UploadRequest object from test case input 'expData [][]byte'.
		var expectedMsgs []*v1.UploadRequest
		expectedMsgs = append(expectedMsgs, &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_Metadata{
				Metadata: &v1.UploadMetadata{
					PartId:   partID,
					Type:     v1.DataType_DATA_TYPE_FILE,
					FileName: filepath.Base(tf.Name()),
				},
			},
		})
		for _, expMsg := range tc.expData {
			expectedMsgs = append(expectedMsgs, &v1.UploadRequest{
				UploadPacket: &v1.UploadRequest_FileContents{
					FileContents: &v1.FileData{
						Data: expMsg,
					},
				},
			})
		}
		time.Sleep(time.Millisecond * 100)

		sut.Close()
		// The mc.sent value should be the same as the expectedMsgs value.
		compareMetadata(t, mc.sent[0].GetMetadata(), expectedMsgs[0].GetMetadata())
		if len(mc.sent) > 1 {
			test.That(t, mc.sent[1:], test.ShouldResemble, expectedMsgs[1:])
		}
	}
}

func TestSensorUploadTabular(t *testing.T) {
	protoMsgTabularStruct := toProto(anyStruct{})

	tests := []struct {
		name    string
		toSend  []*v1.SensorData
		expData []*structpb.Struct
	}{
		{
			name: "One sensor data.",
			toSend: []*v1.SensorData{
				{
					Metadata: &v1.SensorMetadata{},
					Data: &v1.SensorData_Struct{
						Struct: protoMsgTabularStruct,
					},
				},
			},
			expData: []*structpb.Struct{protoMsgTabularStruct},
		},
		{
			name: "A stream of sensor data.",
			toSend: []*v1.SensorData{
				{
					Metadata: &v1.SensorMetadata{},
					Data: &v1.SensorData_Struct{
						Struct: protoMsgTabularStruct,
					},
				},
				{
					Metadata: &v1.SensorMetadata{},
					Data: &v1.SensorData_Struct{
						Struct: protoMsgTabularStruct,
					},
				},
			},
			expData: []*structpb.Struct{protoMsgTabularStruct, protoMsgTabularStruct},
		},
	}

	for _, tc := range tests {
		mc := &mockClient{
			sent: []*v1.UploadRequest{},
			lock: sync.Mutex{},
		}

		// Create temp data capture file.
		tf, err := createTmpDataCaptureFile()
		if err != nil {
			t.Errorf("%s cannot create temporary file to be used for sensorUpload/fileUpload testing: %v", tc.name, err)
		}
		defer os.Remove(tf.Name())

		// First write metadata to file.
		syncMetadata := v1.DataCaptureMetadata{
			ComponentType:    componentType,
			ComponentName:    componentName,
			MethodName:       methodName,
			Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			MethodParameters: nil,
		}
		if _, err := pbutil.WriteDelimited(tf, &syncMetadata); err != nil {
			t.Errorf("%s cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v",
				tc.name, err)
		}

		// Write the data from the test cases into the files to prepare them for reading by the fileUpload function
		for i := range tc.toSend {
			if _, err := pbutil.WriteDelimited(tf, tc.toSend[i]); err != nil {
				t.Errorf("%s cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v",
					tc.name, err)
			}
		}

		sut := newTestSyncer(t, mc, nil)
		sut.Sync([]string{tf.Name()})

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		var expectedMsgs []*v1.UploadRequest
		expectedMsgs = append(expectedMsgs, &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_Metadata{
				Metadata: &v1.UploadMetadata{
					PartId:           partID,
					ComponentType:    componentType,
					ComponentName:    componentName,
					MethodName:       methodName,
					Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
					FileName:         filepath.Base(tf.Name()),
					MethodParameters: nil,
				},
			},
		})
		for _, expMsg := range tc.expData {
			expectedMsgs = append(expectedMsgs, &v1.UploadRequest{
				UploadPacket: &v1.UploadRequest_SensorContents{
					SensorContents: &v1.SensorData{
						Data: &v1.SensorData_Struct{
							Struct: expMsg,
						},
					},
				},
			})
		}

		// The mc.sent value should be the same as the expectedMsgs value.
		time.Sleep(100 * time.Millisecond)
		sut.Close()
		compareUploadRequests(t, true, mc.sent, expectedMsgs)
	}
}

func TestSensorUploadBinary(t *testing.T) {
	msgEmpty := []byte("")
	msgContents := []byte("This is a message. This message is part of testing in datamanager service in RDK.")
	msgBin1 := []byte("Robots are really cool.")
	msgBin2 := []byte("This work is helping develop the robotics space.")
	msgBin3 := []byte("This message is used for testing.")

	tests := []struct {
		name    string
		toSend  [][]byte
		expData [][]byte
	}{
		{
			name:    "empty",
			toSend:  [][]byte{msgEmpty},
			expData: [][]byte{msgEmpty},
		},
		{
			name:    "one binary sensor data reading",
			toSend:  [][]byte{msgContents},
			expData: [][]byte{msgContents},
		},
		{
			name:    "stream of binary sensor data readings",
			toSend:  [][]byte{msgBin1, msgBin2, msgBin3},
			expData: [][]byte{msgBin1, msgBin2, msgBin3},
		},
	}

	for _, tc := range tests {
		mc := &mockClient{
			sent: []*v1.UploadRequest{},
			lock: sync.Mutex{},
		}

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

		// Upload the contents from the created file.
		sut := newTestSyncer(t, mc, nil)
		sut.Sync([]string{tf.Name()})

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		var expectedMsgs []*v1.UploadRequest
		expectedMsgs = append(expectedMsgs, &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_Metadata{
				Metadata: &v1.UploadMetadata{
					PartId:           partID,
					ComponentType:    componentType,
					ComponentName:    componentName,
					MethodName:       methodName,
					Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
					FileName:         filepath.Base(tf.Name()),
					MethodParameters: nil,
				},
			},
		})
		for _, expMsg := range tc.expData {
			expectedMsgs = append(expectedMsgs, &v1.UploadRequest{
				UploadPacket: &v1.UploadRequest_SensorContents{
					SensorContents: &v1.SensorData{
						Data: &v1.SensorData_Binary{
							Binary: expMsg,
						},
					},
				},
			})
		}

		// The mc.sent value should be the same as the expectedMsgs value.
		time.Sleep(100 * time.Millisecond)
		sut.Close()
		compareUploadRequests(t, true, mc.sent, expectedMsgs)
	}
}

// Validates that for some captureDir, files are uploaded exactly once.
func TestUploadsOnce(t *testing.T) {
	mc := &mockClient{
		sent: []*v1.UploadRequest{},
		lock: sync.Mutex{},
	}
	sut := newTestSyncer(t, mc, nil)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile("", "whatever2")
	defer os.Remove(file2.Name())
	// Immediately try to Sync same files many times.
	for i := 1; i < 10; i++ {
		sut.Sync([]string{file1.Name(), file2.Name()})
	}

	// Verify upload was only called twice.
	time.Sleep(time.Millisecond * 100)
	sut.Close()
	test.That(t, len(mc.sent), test.ShouldEqual, 2)
	test.That(t, mc.sent[0], test.ShouldNotEqual, mc.sent[1])
}

func TestUploadExponentialRetry(t *testing.T) {
	// Set retry related global vars to faster values for test.
	initialWaitTime = time.Millisecond * 25
	maxRetryInterval = time.Millisecond * 150
	// Define an uploadFunc that fails 4 times then succeeds on its 5th attempt.
	failureCount := 0
	successCount := 0
	callTimes := make(map[int]time.Time)
	uploadFunc := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string, partID string) error {
		callTimes[failureCount+successCount] = time.Now()
		if failureCount >= 4 {
			successCount++
			return nil
		}
		failureCount++
		return errors.New("fail for the first 4 tries, then succeed")
	}
	mc := &mockClient{
		sent: []*v1.UploadRequest{},
		lock: sync.Mutex{},
	}
	sut := newTestSyncer(t, mc, uploadFunc)

	// Sync file.
	file1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(file1.Name())
	sut.Sync([]string{file1.Name()})

	// Let it run.
	time.Sleep(time.Second)
	sut.Close()

	// Test that upload failed 4 times then succeeded once.
	test.That(t, failureCount, test.ShouldEqual, 4)
	test.That(t, successCount, test.ShouldEqual, 1)

	// Test that exponential increase happens.
	// First retry should wait initialWaitTime
	// Give some leeway so small variations in timing don't cause test failures.
	marginOfError := time.Millisecond * 20
	test.That(t, callTimes[1].Sub(callTimes[0]), test.ShouldAlmostEqual, initialWaitTime, marginOfError)

	// Then increase by a factor of retryExponentialFactor each time
	test.That(t, callTimes[2].Sub(callTimes[1]), test.ShouldAlmostEqual,
		initialWaitTime*time.Duration(retryExponentialFactor), marginOfError)
	test.That(t, callTimes[3].Sub(callTimes[2]), test.ShouldAlmostEqual,
		initialWaitTime*time.Duration(retryExponentialFactor*retryExponentialFactor), marginOfError)

	// ... but not increase past maxRetryInterval.
	test.That(t, callTimes[4].Sub(callTimes[3]), test.ShouldAlmostEqual, maxRetryInterval, marginOfError)
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

package datamanager

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

// implements DataSyncService_UploadClient.
type mockClient struct {
	sent []v1.UploadRequest
	grpc.ClientStream
}

func (m *mockClient) Send(req *v1.UploadRequest) error {
	m.sent = append(m.sent, *req)
	return nil
}

func (m *mockClient) CloseAndRecv() (*v1.UploadResponse, error) {
	return &v1.UploadResponse{}, nil
}

func (m *mockClient) Context() context.Context {
	return context.TODO()
}

type anyStruct struct {
	fieldOne   bool
	fieldTwo   int
	fieldThree string
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
func writeBinarySensorData(f *os.File, toWrite [][]byte) (int, error) {
	countBytesWritten := 0
	for _, bytes := range toWrite {
		msg := &v1.SensorData{
			Data: &v1.SensorData_Binary{
				Binary: bytes,
			},
		}
		bytesWritten, err := pbutil.WriteDelimited(f, msg)
		if err != nil {
			return countBytesWritten, err
		}
		countBytesWritten += bytesWritten
	}
	return countBytesWritten, nil
}

// Compares UploadRequests (which hold either binary or tabular data).
func compareUploadRequests(t *testing.T, isTabular bool, actual []v1.UploadRequest, expected []v1.UploadRequest) {
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
	test.That(t, actualMetadata.PartName, test.ShouldEqual, expectedMetadata.PartName)
	test.That(t, actualMetadata.ComponentName, test.ShouldEqual, expectedMetadata.ComponentName)
	test.That(t, actualMetadata.MethodName, test.ShouldEqual, expectedMetadata.MethodName)
	test.That(t, actualMetadata.Type, test.ShouldEqual, expectedMetadata.Type)
}

// Builds syncer used in tests.
func newTestSyncer(t *testing.T, uploadFn uploadFn) syncer {
	t.Helper()
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	l := golog.NewTestLogger(t)

	return syncer{
		logger: l,
		progressTracker: progressTracker{
			lock: &sync.Mutex{},
			m:    make(map[string]struct{}),
		},
		uploadFn:   uploadFn,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFn,
	}
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
		// Create mockClient that will be sending requests, this mock will have an UploadRequest slice that will
		// contain the UploadRequests that are created by the data contained in files.
		mc := &mockClient{
			sent: []v1.UploadRequest{},
		}

		// Create temp dir and file in that dir to be used as examples of reading data from the files into buffers
		// (and finally to have that data be uploaded) to the cloud.
		td, err := ioutil.TempDir("", "temp-dir")
		if err != nil {
			t.Errorf("%v: cannot create temporary directory to be used for sensorUpload/fileUpload testing", tc.name)
		}
		tf, err := ioutil.TempFile(td, tc.name)
		if err != nil {
			t.Errorf("%v: cannot create temporary file to be used for sensorUpload/fileUpload testing", tc.name)
		}
		defer os.Remove(tf.Name())
		defer os.Remove(td)

		// Write the data from test cases into the temp file to prepare for reading by the fileUpload function.
		if _, err := tf.Write(tc.toSend); err != nil {
			t.Errorf("%v: cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing", tc.name)
		}

		if err := viamUpload(context.TODO(), mc, tf.Name()); err != nil {
			t.Errorf("%v: cannot upload file", tc.name)
		}

		// Create []v1.UploadRequest object from test case input 'expData [][]byte'.
		expectedMsgs := []v1.UploadRequest{}
		expectedMsgs = append(expectedMsgs, v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_Metadata{
				Metadata: &v1.UploadMetadata{
					PartName:      hardCodePartName,
					ComponentName: hardCodeComponentName,
					MethodName:    hardCodeMethodName,
					Type:          v1.DataType_DATA_TYPE_FILE,
					FileName:      filepath.Base(tf.Name()),
				},
			},
		})
		for _, expMsg := range tc.expData {
			expectedMsgs = append(expectedMsgs, v1.UploadRequest{
				UploadPacket: &v1.UploadRequest_FileContents{
					FileContents: &v1.FileData{
						Data: expMsg,
					},
				},
			})
		}

		// The mc.sent value should be the same as the expectedMsgs value.
		compareMetadata(t, mc.sent[0].GetMetadata(), expectedMsgs[0].GetMetadata())
		if len(mc.sent) > 1 {
			test.That(t, mc.sent[1:], test.ShouldResemble, expectedMsgs[1:])
		}
	}
}

func TestSensorUploadTabular(t *testing.T) {
	protoMsgTabularStruct := toProto(
		anyStruct{
			fieldOne:   true,
			fieldTwo:   16,
			fieldThree: "Viam",
		})

	tests := []struct {
		name    string
		toSend  []*v1.SensorData
		expData []*structpb.Struct
	}{
		{
			name: "any struct",
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
			name: "stream of tabular sensor data",
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
		// Create mockClient that will be sending requests, this mock will have an UploadRequest slice that will
		// contain the UploadRequests that are created by the data contained in files.
		mc := &mockClient{
			sent: []v1.UploadRequest{},
		}

		// Create temp dir and file in that dir to be used as examples of reading data from the files into buffers
		// (and finally to have that data be uploaded) to the cloud
		td, err := ioutil.TempDir("", "temp-dir")
		if err != nil {
			t.Errorf("%v cannot create temporary directory to be used for sensorUpload/fileUpload testing", tc.name)
		}
		tf, err := ioutil.TempFile(td, tc.name)
		if err != nil {
			t.Errorf("%v cannot create temporary file to be used for sensorUpload/fileUpload testing", tc.name)
		}
		defer os.Remove(tf.Name())
		defer os.Remove(td)

		// Write the data from the test cases into the files to prepare them for reading by the fileUpload function
		for i := range tc.toSend {
			if _, err := pbutil.WriteDelimited(tf, tc.toSend[i]); err != nil {
				t.Errorf("%v cannot write protobuf struct to temporary file as part of setup for sensorUpload testing",
					tc.name)
			}
		}

		if err := viamUpload(context.TODO(), mc, tf.Name()); err != nil {
			t.Errorf("%v cannot upload file", tc.name)
		}

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		expectedMsgs := []v1.UploadRequest{}
		expectedMsgs = append(expectedMsgs, v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_Metadata{
				Metadata: &v1.UploadMetadata{
					PartName:      hardCodePartName,
					ComponentName: hardCodeComponentName,
					MethodName:    hardCodeMethodName,
					Type:          v1.DataType_DATA_TYPE_TABULAR_SENSOR,
					FileName:      filepath.Base(tf.Name()),
				},
			},
		})
		for _, expMsg := range tc.expData {
			expectedMsgs = append(expectedMsgs, v1.UploadRequest{
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
		// Create mockClient that will be sending requests, this mock will have an UploadRequest slice that will
		// contain the UploadRequests that are created by the data contained in files.
		mc := &mockClient{
			sent: []v1.UploadRequest{},
		}

		// Create temp dir and file in that dir to be used as examples of reading data from the files into
		// buffers (and finally to have that data be uploaded) to the cloud
		td, err := ioutil.TempDir("", "temp-dir")
		if err != nil {
			t.Errorf("%v cannot create temporary directory to be used for sensorUpload/fileUpload testing", tc.name)
		}
		tf, err := ioutil.TempFile(td, tc.name)
		if err != nil {
			t.Errorf("%v cannot create temporary file to be used for sensorUpload/fileUpload testing", tc.name)
		}
		defer os.Remove(tf.Name())
		defer os.Remove(td)

		// Write the data from the test cases into the files to prepare them for reading by the sensorUpload function.
		if _, err := writeBinarySensorData(tf, tc.toSend); err != nil {
			t.Errorf("%v cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing", tc.name)
		}

		// Upload the contents from the created file.
		if err := viamUpload(context.TODO(), mc, tf.Name()); err != nil {
			t.Errorf("%v cannot upload file", tc.name)
		}

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		expectedMsgs := []v1.UploadRequest{}
		expectedMsgs = append(expectedMsgs, v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_Metadata{
				Metadata: &v1.UploadMetadata{
					PartName:      hardCodePartName,
					ComponentName: hardCodeComponentName,
					MethodName:    hardCodeMethodName,
					Type:          v1.DataType_DATA_TYPE_BINARY_SENSOR,
					FileName:      filepath.Base(tf.Name()),
				},
			},
		})
		for _, expMsg := range tc.expData {
			expectedMsgs = append(expectedMsgs, v1.UploadRequest{
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
		compareUploadRequests(t, true, mc.sent, expectedMsgs)
	}
}

// Validates that for some captureDir, files are enqueued and uploaded exactly once.
func TestQueuesAndUploadsOnce(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

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
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

func TestUploadExponentialRetry(t *testing.T) {
	dir := t.TempDir()
	// Set retry related global vars to faster values for test.
	initialWaitTime = time.Millisecond * 25
	maxRetryInterval = time.Millisecond * 150
	// Define an uploadFunc that fails 4 times then succeeds on its 5th attempt.
	failureCount := 0
	successCount := 0
	callTimes := make(map[int]time.Time)
	uploadFunc := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error {
		callTimes[failureCount+successCount] = time.Now()
		if failureCount >= 4 {
			successCount++
			return nil
		}
		failureCount++
		return errors.New("fail for the first 4 tries, then succeed")
	}
	sut := newTestSyncer(t, uploadFunc)

	// Sync file.
	file1, _ := ioutil.TempFile(dir, "whatever")
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

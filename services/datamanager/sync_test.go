package datamanager

import (
	"context"
	"io/ioutil"
	"os"
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

type empty struct{}

type allLiterals struct {
	Bool   bool
	Float  float64
	Int    int
	Int64  int64
	String string
}

type allArrays struct {
	BoolArray   []bool
	FloatArray  []float64
	IntArray    []int
	Int64Array  []int64
	StringArray []string
}

type metaStruct struct {
	AllArrays   allArrays
	AllLiterals allLiterals
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
		// fmt.Printf("num bytes written: %d\n", bytesWritten)
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
	test.That(t, len(actual), test.ShouldEqual, len(expected))
	if !isTabular {
		for i, uploadRequest := range actual {
			a := uploadRequest.GetSensorContents().GetBinary()
			e := expected[i].GetSensorContents().GetBinary()
			test.That(t, a, test.ShouldResemble, e)
		}
	} else {
		for i, uploadRequest := range actual {
			a := uploadRequest.GetSensorContents().GetStruct()
			e := actual[i].GetSensorContents().GetStruct()
			test.That(t, a, test.ShouldResemble, e)
		}
	}
}

// Builds syncer used in tests.
func newTestSyncer(t *testing.T, uploadFn uploadFn) syncer {
	t.Helper()
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	captureDir := t.TempDir()
	syncQueue := t.TempDir()
	l := golog.NewTestLogger(t)

	return syncer{
		captureDir:    captureDir,
		syncQueue:     syncQueue,
		logger:        l,
		queueWaitTime: time.Nanosecond,
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
			name:    "not empty",
			toSend:  msgContents,
			expData: [][]byte{msgContents},
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
			t.Errorf("%v cannot create temporary directory to be used for sensorUpload/fileUpload testing", tc.name)
		}
		tf, err := ioutil.TempFile(td, tc.name)
		if err != nil {
			t.Errorf("%v cannot create temporary file to be used for sensorUpload/fileUpload testing", tc.name)
		}
		defer os.Remove(tf.Name())
		defer os.Remove(td)

		// Write the data from the test cases into the files to prepare them for reading by the fileUpload function.
		if _, err := tf.Write(tc.toSend); err != nil {
			t.Errorf("%v cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing", tc.name)
		}

		if err := fileUpload(context.TODO(), mc, tf.Name()); err != nil {
			t.Errorf("%v cannot upload file", tc.name)
		}

		// Create []v1.UploadRequest object from test case input 'expData [][]byte'.
		expectedMsgs := []v1.UploadRequest{}
		for _, expMsg := range tc.expData {
			expectedMsgs = append(expectedMsgs, v1.UploadRequest{
				UploadPacket: &v1.UploadRequest_FileContents{
					FileContents: &v1.FileData{
						Data: expMsg,
					},
				},
			})

		}

		// The mc.sent value should be the same as the tc.expMsg value.
		test.That(t, mc.sent, test.ShouldResemble, expectedMsgs)
	}
}

func TestSensorUploadTabular(t *testing.T) {
	protoMsgTabularEmpty := toProto(empty{})
	protoMsgTabularNestedStructs := toProto(metaStruct{
		AllArrays: allArrays{
			BoolArray:   []bool{false, true},
			FloatArray:  []float64{12.4, 0.9},
			IntArray:    []int{7, 9},
			Int64Array:  []int64{3, 2},
			StringArray: []string{"Viam is cool.", "The interns are great!"},
		},
		AllLiterals: allLiterals{
			Bool:   false,
			Float:  12.4,
			Int:    7,
			Int64:  3,
			String: "Viam is cool.",
		},
	})

	tests := []struct {
		name    string
		toSend  *v1.SensorData
		expData []*structpb.Struct
	}{
		{
			name: "empty struct",
			toSend: &v1.SensorData{
				Metadata: &v1.SensorMetadata{},
				Data: &v1.SensorData_Struct{
					Struct: protoMsgTabularEmpty,
				},
			},
			expData: []*structpb.Struct{},
		},
		{
			name: "structs with each literal, arrays, and nested structs",
			toSend: &v1.SensorData{
				Metadata: &v1.SensorMetadata{},
				Data: &v1.SensorData_Struct{
					Struct: protoMsgTabularNestedStructs,
				},
			},
			expData: []*structpb.Struct{protoMsgTabularNestedStructs},
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
		if _, err := pbutil.WriteDelimited(tf, tc.toSend); err != nil {
			t.Errorf("%v cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing", tc.name)
		}

		if err := sensorUpload(context.TODO(), mc, tf.Name()); err != nil {
			t.Errorf("%v cannot upload file", tc.name)
		}

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		expectedMsgs := []v1.UploadRequest{}
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

		// The mc.sent value should be the same as the expectedMsgs value
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
			name:    "not empty",
			toSend:  [][]byte{msgContents},
			expData: [][]byte{msgContents},
		},
		{
			name:    "empty",
			toSend:  [][]byte{msgEmpty},
			expData: [][]byte{msgEmpty},
		},
		{
			name:    "multiple sensor data readings",
			toSend:  [][]byte{msgBin1, msgBin2, msgBin3},
			expData: [][]byte{msgBin1, msgBin2, msgBin3},
		},
	}

	for _, tc := range tests {
		// Create mockClient that will be sending requests,
		// this mock will have an UploadRequest slice that
		// will contain the UploadRequests that are created
		// by the data contained in files.
		mc := &mockClient{
			sent: []v1.UploadRequest{},
		}

		// Create temp dir and file in that dir to be used
		// as examples of reading data from the files into
		// buffers (and finally to have that data be uploaded)
		// to the cloud
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

		// Write the data from the test cases into the files
		// to prepare them for reading by the sensorUpload function

		// NOT SURE IF THIS WORKS
		if _, err := writeBinarySensorData(tf, tc.toSend); err != nil {
			t.Errorf("%v cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing", tc.name)
		}
		// THIS IS NOT WORKING
		if err := sensorUpload(context.TODO(), mc, tf.Name()); err != nil {
			t.Errorf("%v cannot upload file", tc.name)
		}

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		expectedMsgs := []v1.UploadRequest{}
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

		// The mc.sent value should be the same as the expectedMsgs value
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

	// Start syncer.
	sut.Start()

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever2")
	defer os.Remove(file2.Name())
	err := sut.Enqueue([]string{file1.Name(), file2.Name()})
	test.That(t, err, test.ShouldBeNil)
	// Give it a second to run and upload files.
	time.Sleep(time.Second)

	// Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(sut.syncQueue)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

// Validates that if a syncer is killed after enqueing a file, a new syncer will still pick it up and upload it. This
// is to simulate the case where a robot is killed mid-sync; we still want that sync to resume and finish when it
// turns back on.
func TestRecoversAfterKilled(t *testing.T) {
	var uploadCount uint64
	uploadFn := func(ctx context.Context, client v1.DataSyncService_UploadClient, path string) error {
		atomic.AddUint64(&uploadCount, 1)
		_ = os.Remove(path)
		return nil
	}
	sut := newTestSyncer(t, uploadFn)

	// Put a file in syncDir; this simulates a file that was enqueued by some previous syncer.
	file1, _ := ioutil.TempFile(sut.syncQueue, "whatever")
	defer os.Remove(file1.Name())

	// Put a file in captureDir; this simulates a file that was written but not yet queued by some previous syncer.
	// It should be synced even if it is not specified in the list passed to Enqueue.
	file2, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file2.Name())

	// Start syncer, let it run for a second.
	sut.Start()
	err := sut.Enqueue([]string{})
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(time.Second)

	// Verify enqueued files were uploaded.
	filesInQueue, err := ioutil.ReadDir(sut.syncQueue)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}
	// Verify previously captured but not queued files were uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(sut.captureDir)
	if err != nil {
		t.Fatalf("failed to list files in syncDir")
	}
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, atomic.LoadUint64(&uploadCount), test.ShouldEqual, 2)
	sut.Close()
}

func TestUploadExponentialRetry(t *testing.T) {
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

	// Put a file to be synced in captureDir.
	file1, _ := ioutil.TempFile(sut.captureDir, "whatever")
	defer os.Remove(file1.Name())

	// Start syncer and let it run.
	initialWaitTime = time.Millisecond * 25
	maxRetryInterval = time.Millisecond * 150
	sut.Start()
	err := sut.Enqueue([]string{})
	test.That(t, err, test.ShouldBeNil)
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

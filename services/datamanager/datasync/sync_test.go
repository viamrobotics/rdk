package datasync

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"github.com/viamrobotics/app/datasync"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	syncWaitTime = 100 * time.Millisecond
)

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
		t.Log(tc.name)

		// Register mock datasync service with a mock server.
		logger, _ := golog.NewObservedTestLogger(t)
		mockService := getMockService()
		rpcServer := buildAndStartLocalServer(t, logger, mockService)
		defer func() {
			err := rpcServer.Stop()
			test.That(t, err, test.ShouldBeNil)
		}()

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

		conn, err := getLocalServerConn(rpcServer, logger)
		test.That(t, err, test.ShouldBeNil)
		client := NewClient(conn)
		sut, err := NewManager(logger, partID, client, conn)
		test.That(t, err, test.ShouldBeNil)
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
		time.Sleep(syncWaitTime)

		sut.Close()
		// The mc.sent value should be the same as the expectedMsgs value.
		compareMetadata(t, mockService.getUploadRequests()[0].GetMetadata(), expectedMsgs[0].GetMetadata())
		test.That(t, len(mockService.getUploadRequests()), test.ShouldEqual, len(expectedMsgs))
		actual := mockService.getUploadRequests()[1:]
		if len(expectedMsgs) > 1 {
			for i, exp := range expectedMsgs[1:] {
				test.That(t, string(actual[i].GetFileContents().GetData()),
					test.ShouldEqual, string(exp.GetFileContents().GetData()))
			}
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
		t.Log(tc.name)

		// Create temp data capture file.
		tf, err := createTmpDataCaptureFile()
		if err != nil {
			t.Errorf("%s cannot create temporary file to be used for sensorUpload/fileUpload testing: %v", tc.name, err)
		}
		defer os.Remove(tf.Name())

		// First write metadata to file.
		captureMetadata := v1.DataCaptureMetadata{
			ComponentType:    componentType,
			ComponentName:    componentName,
			MethodName:       methodName,
			Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			MethodParameters: nil,
		}
		if _, err := pbutil.WriteDelimited(tf, &captureMetadata); err != nil {
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

		// Register mock datasync service with a mock server.
		logger, _ := golog.NewObservedTestLogger(t)
		mockService := getMockService()
		rpcServer := buildAndStartLocalServer(t, logger, mockService)
		defer func() {
			err := rpcServer.Stop()
			test.That(t, err, test.ShouldBeNil)
		}()

		conn, err := getLocalServerConn(rpcServer, logger)
		test.That(t, err, test.ShouldBeNil)
		client := NewClient(conn)
		sut, err := NewManager(logger, partID, client, conn)
		test.That(t, err, test.ShouldBeNil)
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
		time.Sleep(syncWaitTime)
		sut.Close()
		compareUploadRequests(t, true, mockService.getUploadRequests(), expectedMsgs)
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
		t.Log(tc.name)

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
		// Register mock datasync service with a mock server.
		logger, _ := golog.NewObservedTestLogger(t)
		mockService := getMockService()
		rpcServer := buildAndStartLocalServer(t, logger, mockService)
		defer func() {
			err := rpcServer.Stop()
			test.That(t, err, test.ShouldBeNil)
		}()

		conn, err := getLocalServerConn(rpcServer, logger)
		test.That(t, err, test.ShouldBeNil)
		client := NewClient(conn)
		sut, err := NewManager(logger, partID, client, conn)
		test.That(t, err, test.ShouldBeNil)
		sut.Sync([]string{tf.Name()})

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		expectedMsgs := buildBinaryUploadRequests(tc.expData, filepath.Base(tf.Name()))

		// The mc.sent value should be the same as the expectedMsgs value.
		time.Sleep(syncWaitTime)
		sut.Close()
		compareUploadRequests(t, true, mockService.getUploadRequests(), expectedMsgs)
	}
}

// Validates that for some captureDir, files are uploaded exactly once.
func TestUploadsOnce(t *testing.T) {
	// Register mock datasync service with a mock server.
	logger, _ := golog.NewObservedTestLogger(t)
	mockService := getMockService()
	rpcServer := buildAndStartLocalServer(t, logger, mockService)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	conn, err := getLocalServerConn(rpcServer, logger)
	test.That(t, err, test.ShouldBeNil)
	client := NewClient(conn)
	sut, err := NewManager(logger, partID, client, conn)
	test.That(t, err, test.ShouldBeNil)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile("", "whatever")
	file2, _ := ioutil.TempFile("", "whatever2")

	// Immediately try to Sync same files many times.
	for i := 1; i < 10; i++ {
		sut.Sync([]string{file1.Name(), file2.Name()})
	}

	// Verify upload was only called twice.
	time.Sleep(syncWaitTime)
	sut.Close()
	test.That(t, mockService.callCount.Load(), test.ShouldEqual, 2)

	// Verify the two upload calls were made on different files.
	var metadatas []*v1.UploadMetadata
	for _, ur := range mockService.getUploadRequests() {
		if ur.GetMetadata() != nil {
			metadatas = append(metadatas, ur.GetMetadata())
		}
	}
	test.That(t, len(metadatas), test.ShouldEqual, 2)
	test.That(t, metadatas[0].GetFileName(), test.ShouldNotEqual, metadatas[1].GetFileName())

	// Verify that the files were deleted after upload.
	_, err = os.Stat(file1.Name())
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(file2.Name())
	test.That(t, err, test.ShouldNotBeNil)
}

func TestUploadExponentialRetry(t *testing.T) {
	// Set retry related global vars to faster values for test.
	initialWaitTime = time.Millisecond * 50
	maxRetryInterval = time.Millisecond * 150
	uploadChunkSize = 10
	// Register mock datasync service with a mock server.
	logger, _ := golog.NewObservedTestLogger(t)

	tests := []struct {
		name             string
		err              error
		waitTime         time.Duration
		expCallCount     int32
		shouldStillExist bool
	}{
		{
			name:         "Retryable errors should be retried",
			err:          errors.New("literally any error here"),
			waitTime:     time.Second,
			expCallCount: 4,
		},
		{
			name:             "Non-retryable errors should not be retried",
			err:              datasync.ErrInvalidRequest,
			waitTime:         time.Millisecond * 300,
			expCallCount:     1,
			shouldStillExist: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Build a mock service that fails 3 times before succeeding.
			mockService := getMockService()
			mockService.failUntilIndex = 3
			mockService.errorToReturn = tc.err
			rpcServer := buildAndStartLocalServer(t, logger, mockService)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			conn, err := getLocalServerConn(rpcServer, logger)
			test.That(t, err, test.ShouldBeNil)
			client := NewClient(conn)
			sut, err := NewManager(logger, partID, client, conn)
			test.That(t, err, test.ShouldBeNil)

			// Start file sync.
			file1, _ := ioutil.TempFile("", "whatever")
			defer os.Remove(file1.Name())
			_, _ = file1.Write([]byte("this is some amount of content greater than 10"))
			sut.Sync([]string{file1.Name()})

			// Let it run so it can retry (or not).
			time.Sleep(tc.waitTime)
			sut.Close()

			// Validate that the client called Upload the correct number of times, and that the file was not deleted.
			test.That(t, mockService.callCount.Load(), test.ShouldEqual, tc.expCallCount)
			_, err = os.Stat(file1.Name())
			exists := !errors.Is(err, os.ErrNotExist)
			test.That(t, exists, test.ShouldEqual, tc.shouldStillExist)
		})
	}

	// Build a mock service that fails 3 times before succeeding.
	mockService := getMockService()
	mockService.failUntilIndex = 3
	rpcServer := buildAndStartLocalServer(t, logger, mockService)
	uploadChunkSize = 10
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	conn, err := getLocalServerConn(rpcServer, logger)
	test.That(t, err, test.ShouldBeNil)
	client := NewClient(conn)
	sut, err := NewManager(logger, partID, client, conn)
	test.That(t, err, test.ShouldBeNil)

	// Sync file.
	file1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(file1.Name())
	_, _ = file1.Write([]byte("this is some amount of content greater than 10"))
	sut.Sync([]string{file1.Name()})

	// Let it run.
	time.Sleep(time.Second)
	sut.Close()

	// Validate that the client called Upload repeatedly.
	test.That(t, mockService.callCount.Load(), test.ShouldEqual, 4)
	_ = conn.Close()
	_ = rpcServer.Stop()

	// Test that non-retryable errors are not retried.
	mockService = getMockService()
	mockService.failUntilIndex = 3
	mockService.errorToReturn = datasync.ErrEmptyStream
	rpcServer = buildAndStartLocalServer(t, logger, mockService)
	uploadChunkSize = 10
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()
	conn, err = getLocalServerConn(rpcServer, logger)
	test.That(t, err, test.ShouldBeNil)
	client = NewClient(conn)
	sut, err = NewManager(logger, partID, client, conn)
	test.That(t, err, test.ShouldBeNil)

	// Sync file.
	file2, _ := ioutil.TempFile("", "whatever2")
	defer os.Remove(file2.Name())
	_, _ = file1.Write([]byte("this is some amount of content greater than 10"))
	sut.Sync([]string{file2.Name()})

	// Let it run.
	time.Sleep(time.Second)
	sut.Close()

	// Validate that the client called Upload repeatedly.
	test.That(t, mockService.callCount.Load(), test.ShouldEqual, 1)
}

func TestPartialUpload(t *testing.T) {
	initialWaitTime = time.Minute
	msg1 := []byte("viam")
	msg2 := []byte("robotics")
	msg3 := []byte("builds cool software")
	msg4 := toProto(anyStruct{})
	msg5 := toProto(anyStruct{Field1: false})
	msg6 := toProto(anyStruct{Field1: true, Field2: 2020, Field3: "viam"})

	tests := []struct {
		name        string
		cancelIndex int32
		toSend      []*v1.SensorData
		expUR       []*v1.UploadRequest
		dataType    v1.DataType
	}{
		{
			// TODO: add expected upload requests
			name: "Binary upload of non-empty file should resume from last point if it is " +
				"canceled.",
			toSend:      createBinarySensorData([][]byte{msg1, msg2, msg3}),
			cancelIndex: 2,
			dataType:    v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Binary upload of empty file should not upload anything when it is started nor if it " +
				"is resumed.",
			toSend:      []*v1.SensorData{},
			cancelIndex: 0,
			dataType:    v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Binary upload with no more messages to send after it's canceled should not upload " +
				"anything after resuming.",
			toSend:      createBinarySensorData([][]byte{msg1, msg2}),
			cancelIndex: 2,
			dataType:    v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Binary upload that is interrupted before sending a single message should resume and send all" +
				"messages.",
			toSend:      createBinarySensorData([][]byte{msg1, msg2}),
			cancelIndex: 0,
			dataType:    v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Tabular upload of non-empty file should resume from last point if it is" +
				"canceled.",
			toSend:      createTabularSensorData([]*structpb.Struct{msg4, msg5, msg6}),
			cancelIndex: 2,
			dataType:    v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		},
		{
			name: "Tabular upload of empty file should not upload anything when it is started nor if it " +
				"is resumed.",
			toSend:      createTabularSensorData([]*structpb.Struct{}),
			cancelIndex: 0,
			dataType:    v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// TODO in test:
			//      - build mock server that cancels after receiving cancelIndex messages
			//      - verify expected messages before + after cancel

			// Create temp data capture file.
			f, err := createTmpDataCaptureFile()
			test.That(t, err, test.ShouldBeNil)
			// First write metadata to file.
			captureMetadata := v1.DataCaptureMetadata{
				ComponentType:    componentType,
				ComponentName:    componentName,
				MethodName:       methodName,
				Type:             tc.dataType,
				MethodParameters: nil,
			}
			if _, err := pbutil.WriteDelimited(f, &captureMetadata); err != nil {
				t.Errorf("cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v", err)
			}

			// Next write sensor data to file.
			if err := writeSensorData(f, tc.toSend); err != nil {
				t.Errorf("%s: cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing: %v", tc.name, err)
			}

			// Stand up mock server.
			// Register mock datasync service with a mock server.
			logger, _ := golog.NewObservedTestLogger(t)
			mockService := getMockService()
			mockService.failAtIndex = tc.cancelIndex
			rpcServer := buildAndStartLocalServer(t, logger, mockService)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			conn, err := getLocalServerConn(rpcServer, logger)
			test.That(t, err, test.ShouldBeNil)
			client := NewClient(conn)
			sut, err := NewManager(logger, partID, client, conn)
			test.That(t, err, test.ShouldBeNil)
			sut.Sync([]string{f.Name()})
			time.Sleep(syncWaitTime)
			sut.Close()

			// TODO: validate mockService.getUploadRequests for indexes 0:tc.cancelIndex

			// Restart.
			mockService = getMockService()
			rpcServer = buildAndStartLocalServer(t, logger, mockService)
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()
			conn, err = getLocalServerConn(rpcServer, logger)
			test.That(t, err, test.ShouldBeNil)
			client = NewClient(conn)
			sut, err = NewManager(logger, partID, client, conn)
			test.That(t, err, test.ShouldBeNil)
			sut.Sync([]string{f.Name()})
			time.Sleep(syncWaitTime)
			sut.Close()

			// TODO: validate mockService.getUploadRequests for indexes tc.cancelIndex:

			// TODO: Validate progress files do not exist.
		})
	}
}

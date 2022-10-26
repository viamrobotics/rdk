package datasync

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

const (
	syncWaitTime = 500 * time.Millisecond
)

func TestFileUpload(t *testing.T) {
	uploadChunkSize = 10
	msgContents := []byte("This is part of testing in datamanager service in RDK.")

	tests := []struct {
		name    string
		toSend  []byte
		expData [][]byte
	}{
		{
			name:    "Empty file should not send only metadata.",
			toSend:  []byte(""),
			expData: [][]byte{},
		},
		{
			name:   "Non empty file should successfully send its content in chunks.",
			toSend: msgContents,
			expData: [][]byte{
				msgContents[:10], msgContents[10:20], msgContents[20:30], msgContents[30:40],
				msgContents[40:50], msgContents[50:],
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			tf, err := os.CreateTemp("", "")
			test.That(t, err, test.ShouldBeNil)
			defer os.Remove(tf.Name())

			// Write the data from test cases into the temp file to prepare for reading by the fileUpload function.
			_, err = tf.Write(tc.toSend)
			test.That(t, err, test.ShouldBeNil)

			conn, err := getLocalServerConn(rpcServer, logger)
			test.That(t, err, test.ShouldBeNil)
			client := NewClient(conn)
			sut, err := NewManager(logger, partID, client, conn)
			test.That(t, err, test.ShouldBeNil)
			sut.Sync([]string{tf.Name()})
			time.Sleep(syncWaitTime)
			sut.Close()

			// Validate the expected messages were sent.
			expectedMsgs := buildFileDataUploadRequests(tc.expData, filepath.Base(tf.Name()))
			test.That(t, len(mockService.getUploadRequests()), test.ShouldEqual, len(expectedMsgs))
			if len(expectedMsgs) > 1 {
				test.That(t, mockService.getUploadRequests()[0].GetMetadata().String(), test.ShouldResemble,
					expectedMsgs[0].GetMetadata().String())
				for i := range expectedMsgs[1:] {
					test.That(t, string(mockService.getUploadRequests()[i].GetFileContents().GetData()),
						test.ShouldEqual, string(expectedMsgs[i].GetFileContents().GetData()))
				}
			}
		})
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
			name:    "One sensor data.",
			toSend:  createTabularSensorData([]*structpb.Struct{protoMsgTabularStruct}),
			expData: []*structpb.Struct{protoMsgTabularStruct},
		},
		{
			name:    "A stream of sensor data.",
			toSend:  createTabularSensorData([]*structpb.Struct{protoMsgTabularStruct, protoMsgTabularStruct}),
			expData: []*structpb.Struct{protoMsgTabularStruct, protoMsgTabularStruct},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp data capture file.
			tmpDir, err := ioutil.TempDir("", "")
			test.That(t, err, test.ShouldBeNil)
			defer os.RemoveAll(tmpDir)
			// First write metadata to file.
			captureMetadata := v1.DataCaptureMetadata{
				ComponentType:    componentType,
				ComponentName:    componentName,
				ComponentModel:   componentModel,
				MethodName:       methodName,
				Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
				MethodParameters: nil,
				FileExtension:    datacapture.GetFileExt(v1.DataType_DATA_TYPE_TABULAR_SENSOR, methodName, nil),
				Tags:             tags,
			}

			f, err := datacapture.NewFile(tmpDir, &captureMetadata)
			test.That(t, err, test.ShouldBeNil)

			// Write the data from the test cases into the files to prepare them for reading by the fileUpload function
			for _, msg := range tc.toSend {
				err := f.WriteNext(msg)
				test.That(t, err, test.ShouldBeNil)
			}
			err = f.Sync()
			test.That(t, err, test.ShouldBeNil)

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
			sut.Sync([]string{f.GetPath()})
			time.Sleep(syncWaitTime)
			sut.Close()

			// Validate the client sent the expected messages.
			expectedMsgs := buildSensorDataUploadRequests(tc.toSend, v1.DataType_DATA_TYPE_TABULAR_SENSOR,
				filepath.Base(f.GetPath()))
			compareTabularUploadRequests(t, mockService.getUploadRequests(), expectedMsgs)
		})
	}
}

func TestSensorUploadBinary(t *testing.T) {
	msgEmpty := []byte("")
	msgContents := []byte("This is a message. This message is part of testing in datamanager service in RDK.")
	msgBin1 := []byte("Robots are really cool.")
	msgBin2 := []byte("This work is helping develop the robotics space.")
	msgBin3 := []byte("This message is used for testing.")

	tests := []struct {
		name   string
		toSend []*v1.SensorData
	}{
		{
			name:   "empty",
			toSend: createBinarySensorData([][]byte{msgEmpty}),
		},
		{
			name:   "one binary sensor data reading",
			toSend: createBinarySensorData([][]byte{msgContents}),
		},
		{
			name:   "stream of binary sensor data readings",
			toSend: createBinarySensorData([][]byte{msgBin1, msgBin2, msgBin3}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp file to be used as examples of reading data from the files into buffers and finally to have
			// that data be uploaded to the cloud
			tmpDir, err := ioutil.TempDir("", "")
			test.That(t, err, test.ShouldBeNil)
			defer os.RemoveAll(tmpDir)
			// First write metadata to file.
			captureMD := v1.DataCaptureMetadata{
				ComponentType:    componentType,
				ComponentName:    componentName,
				ComponentModel:   componentModel,
				MethodName:       methodName,
				Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
				MethodParameters: nil,
				FileExtension:    datacapture.GetFileExt(v1.DataType_DATA_TYPE_BINARY_SENSOR, methodName, nil),
				Tags:             tags,
			}
			f, err := datacapture.NewFile(tmpDir, &captureMD)
			test.That(t, err, test.ShouldBeNil)

			// Write the data from the test cases into the files to prepare them for reading by the sensorUpload function.
			for _, msg := range tc.toSend {
				err := f.WriteNext(msg)
				test.That(t, err, test.ShouldBeNil)
			}
			err = f.Sync()
			test.That(t, err, test.ShouldBeNil)

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
			sut.Sync([]string{f.GetPath()})

			expectedMsgs := buildSensorDataUploadRequests(tc.toSend, v1.DataType_DATA_TYPE_BINARY_SENSOR, filepath.Base(f.GetPath()))

			// The UploadRequests received by our mock service should be the same as the expectedMsgs value.
			time.Sleep(syncWaitTime)
			sut.Close()
			compareTabularUploadRequests(t, mockService.getUploadRequests(), expectedMsgs)
		})
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
	file1, _ := os.CreateTemp("", "whatever")
	file2, _ := os.CreateTemp("", "whatever2")

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
	// TODO: RSDK-565. Make this work. Bidi broke it.
	t.Skip()

	// Set retry related global vars to faster values for test.
	initialWaitTimeMillis.Store(50)
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
			err:              status.Error(codes.InvalidArgument, "bad"),
			waitTime:         time.Second,
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
			file1, _ := os.CreateTemp("", "whatever")
			defer os.Remove(file1.Name())
			_, _ = file1.Write([]byte("this is some amount of content greater than 10"))
			sut.Sync([]string{file1.Name()})

			// Let it run so it can retry (or not).
			time.Sleep(tc.waitTime)
			sut.Close()

			// Validate that the client called Upload the correct number of times, and whether or not the file was
			// deleted.
			test.That(t, mockService.callCount.Load(), test.ShouldEqual, tc.expCallCount)
			_, err = os.Stat(file1.Name())
			exists := !errors.Is(err, os.ErrNotExist)
			test.That(t, exists, test.ShouldEqual, tc.shouldStillExist)
		})
	}
}

func TestPartialUpload(t *testing.T) {
	// TODO: RSDK-640. Make this work. Bidi broke it.
	t.Skip()

	initialWaitTimeMillis.Store(1000 * 60)
	msg1 := []byte("viam")
	msg2 := []byte("robotics")
	msg3 := []byte("builds cool software")
	msg4 := []byte("yup")
	msg5 := []byte("it sure does")
	msg6 := toProto(anyStruct{})
	msg7 := toProto(anyStruct{Field1: false})
	msg8 := toProto(anyStruct{Field1: true, Field2: 2020, Field3: "viam"})
	msg9 := toProto(anyStruct{Field1: true, Field2: 2021})

	tests := []struct {
		name                          string
		ackEveryNSensorDatas          int
		clientCancelAfterNSensorDatas int
		serverErrorAfterNSensorDatas  int32
		dataType                      v1.DataType
		toSend                        []*v1.SensorData
		expSentBeforeRetry            []*v1.SensorData
		expSentAfterRetry             []*v1.SensorData
	}{
		{
			name:                          `Binary upload should resume from last ACKed point if the syncer is closed.`,
			dataType:                      v1.DataType_DATA_TYPE_BINARY_SENSOR,
			ackEveryNSensorDatas:          2,
			clientCancelAfterNSensorDatas: 3,
			toSend:                        createBinarySensorData([][]byte{msg1, msg2, msg3, msg4, msg5}),
			// First two messages should be ACKed, so only 3-5 should be sent after retry.
			expSentBeforeRetry: createBinarySensorData([][]byte{msg1, msg2, msg3}),
			expSentAfterRetry:  createBinarySensorData([][]byte{msg3, msg4, msg5}),
		},
		{
			name:                          `Tabular upload should resume from last ACKed point if the syncer is closed.`,
			dataType:                      v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			ackEveryNSensorDatas:          2,
			clientCancelAfterNSensorDatas: 3,
			toSend:                        createTabularSensorData([]*structpb.Struct{msg6, msg7, msg8, msg9}),
			// First two messages should be ACKed, so only msg8-9 should be sent after retry.
			expSentBeforeRetry: createTabularSensorData([]*structpb.Struct{msg6, msg7, msg8}),
			expSentAfterRetry:  createTabularSensorData([]*structpb.Struct{msg8, msg9}),
		},
		{
			name:                         `Binary upload should resume from last ACKed point after server disconnection.`,
			dataType:                     v1.DataType_DATA_TYPE_BINARY_SENSOR,
			ackEveryNSensorDatas:         2,
			serverErrorAfterNSensorDatas: 3,
			toSend:                       createBinarySensorData([][]byte{msg1, msg2, msg3, msg4, msg5}),
			// First two messages were ACKed, and third was sent but not acked. Only msg3-5 should be sent after retry.
			expSentBeforeRetry: createBinarySensorData([][]byte{msg1, msg2, msg3}),
			expSentAfterRetry:  createBinarySensorData([][]byte{msg3, msg4, msg5}),
		},
		{
			name:                         `Tabular upload should resume from last ACKed point after server disconnection.`,
			dataType:                     v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			ackEveryNSensorDatas:         2,
			serverErrorAfterNSensorDatas: 3,
			toSend:                       createTabularSensorData([]*structpb.Struct{msg6, msg7, msg8, msg9}),
			// First two messages should be ACKed, so only msg8-9 should be sent after retry.
			expSentBeforeRetry: createTabularSensorData([]*structpb.Struct{msg6, msg7, msg8}),
			expSentAfterRetry:  createTabularSensorData([]*structpb.Struct{msg8, msg9}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "")
			test.That(t, err, test.ShouldBeNil)
			defer os.RemoveAll(tmpDir)
			// First write metadata to file.
			captureMetadata := v1.DataCaptureMetadata{
				ComponentType:    componentType,
				ComponentName:    componentName,
				ComponentModel:   componentModel,
				MethodName:       methodName,
				Type:             tc.dataType,
				MethodParameters: nil,
				FileExtension:    datacapture.GetFileExt(tc.dataType, methodName, nil),
				Tags:             tags,
			}
			f, err := datacapture.NewFile(tmpDir, &captureMetadata)
			test.That(t, err, test.ShouldBeNil)

			for _, msg := range tc.toSend {
				err := f.WriteNext(msg)
				test.That(t, err, test.ShouldBeNil)
			}
			err = f.Sync()
			test.That(t, err, test.ShouldBeNil)

			// Build mock service with configured cancel and ack values.
			logger := golog.NewTestLogger(t)
			mockService := getMockService()
			mockService.messagesPerAck = tc.ackEveryNSensorDatas
			if tc.serverErrorAfterNSensorDatas != 0 {
				mockService.failAtIndex = tc.serverErrorAfterNSensorDatas + 1
			}
			if tc.clientCancelAfterNSensorDatas != 0 {
				mockService.clientShutdownIndex = tc.clientCancelAfterNSensorDatas
			}

			// Build and start a local server and client.
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

			// Use channels so that we can ensure that client shutdown occurs at the exact time we are intending to
			// test (e.g. after the server has received clientShutdownIndex messages).
			cancelChannel := make(chan bool)
			doneCancelChannel := make(chan bool)
			mockService.cancelChannel = cancelChannel
			mockService.doneCancelChannel = doneCancelChannel
			go func() {
				<-cancelChannel
				sut.Close()
				doneCancelChannel <- true
			}()
			sut.Sync([]string{f.GetPath()})
			time.Sleep(syncWaitTime)

			// Validate client sent mockService the upload requests we would expect before canceling the upload.
			var expMsgs []*v1.UploadRequest
			var actMsgs []*v1.UploadRequest

			// Build all expected messages from before the Upload was cancelled.
			expMsgs = buildSensorDataUploadRequests(tc.expSentBeforeRetry, tc.dataType, f.GetPath())
			actMsgs = mockService.getUploadRequests()
			compareTabularUploadRequests(t, actMsgs, expMsgs)

			// Restart the client and server and attempt to sync again.
			mockService = getMockService()
			mockService.messagesPerAck = tc.ackEveryNSensorDatas
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
			sut.Sync([]string{f.GetPath()})
			time.Sleep(syncWaitTime)
			sut.Close()

			// Validate client sent mockService the upload requests we would expect after resuming upload.
			expMsgs = buildSensorDataUploadRequests(tc.expSentAfterRetry, tc.dataType, f.GetPath())
			compareTabularUploadRequests(t, mockService.getUploadRequests(), expMsgs)

			// Validate progress file does not exist.
			files, err := ioutil.ReadDir(viamProgressDotDir)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(files), test.ShouldEqual, 0)

			// Validate data capture file does not exist.
			_, err = os.Stat(f.GetPath())
			test.That(t, err, test.ShouldNotBeNil)
		})
	}
}

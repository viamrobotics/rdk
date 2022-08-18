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
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		time.Sleep(syncWaitTime)
		sut.Close()

		// The mc.sent value should be the same as the expectedMsgs value.
		expectedMsgs := buildFileDataUploadRequests(tc.expData, filepath.Base(tf.Name()))
		if len(expectedMsgs) > 1 {
			compareMetadata(t, mockService.getUploadRequests()[0].GetMetadata(), expectedMsgs[0].GetMetadata())
			test.That(t, len(mockService.getUploadRequests()), test.ShouldEqual, len(expectedMsgs))
			actual := mockService.getUploadRequests()[1:]
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
			ComponentModel:   componentModel,
			MethodName:       methodName,
			Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			MethodParameters: nil,
			FileExtension:    datacapture.GetFileExt(v1.DataType_DATA_TYPE_TABULAR_SENSOR, methodName, nil),
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

		// The mc.sent value should be the same as the expectedMsgs value.
		expectedMsgs := buildSensorDataUploadRequests(tc.toSend, v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			filepath.Base(tf.Name()))
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
			ComponentModel:   componentModel,
			MethodName:       methodName,
			Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			MethodParameters: nil,
			FileExtension:    datacapture.GetFileExt(v1.DataType_DATA_TYPE_BINARY_SENSOR, methodName, nil),
		}
		if _, err := pbutil.WriteDelimited(tf, &syncMetadata); err != nil {
			t.Errorf("%s cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v",
				tc.name, err)
		}

		// Write the data from the test cases into the files to prepare them for reading by the sensorUpload function.
		if err := writeSensorData(tf, tc.toSend); err != nil {
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
		expectedMsgs := buildSensorDataUploadRequests(tc.toSend, v1.DataType_DATA_TYPE_BINARY_SENSOR, filepath.Base(tf.Name()))

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
			err:              status.Error(codes.InvalidArgument, "bad"),
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

			// Validate that the client called Upload the correct number of times, and whether or not the file was
			// deleted.
			test.That(t, mockService.callCount.Load(), test.ShouldEqual, tc.expCallCount)
			_, err = os.Stat(file1.Name())
			exists := !errors.Is(err, os.ErrNotExist)
			test.That(t, exists, test.ShouldEqual, tc.shouldStillExist)
		})
	}
}

// TODO: have separate server fail and client fail tests
// TODO: readd all componentmodel stuff
// TODO: make all message counting stuff not include metadata. Or separate them somehow. I feel like it's weird currently,.
func TestPartialUpload(t *testing.T) {
	initialWaitTime = time.Minute
	msg1 := []byte("viam")
	msg2 := []byte("robotics")
	msg3 := []byte("builds cool software")
	msg4 := []byte("yup")
	msg5 := []byte("it sure does")
	msg6 := toProto(anyStruct{})
	msg7 := toProto(anyStruct{Field1: false})
	msg8 := toProto(anyStruct{Field1: true, Field2: 2020, Field3: "viam"})

	tests := []struct {
		name                            string
		sendAckEveryNSensorDataMessages int
		clientCancelAfterNMsgs          int
		serverErrorAfterNMsgs           int32
		dataType                        v1.DataType
		toSend                          []*v1.SensorData
		expSentAfterRetry               []*v1.SensorData
	}{
		{
			name:                            `Binary upload should resume if the syncer context is cancelled.`,
			dataType:                        v1.DataType_DATA_TYPE_BINARY_SENSOR,
			sendAckEveryNSensorDataMessages: 2,
			clientCancelAfterNMsgs:          4,
			serverErrorAfterNMsgs:           -1,
			toSend:                          createBinarySensorData([][]byte{msg1, msg2, msg3, msg4, msg5}),
			expSentAfterRetry:               createBinarySensorData([][]byte{msg3, msg4, msg5}),
		},
		{
			name:                            `Tabular upload should resume if the syncer context is cancelled,.`,
			dataType:                        v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			sendAckEveryNSensorDataMessages: 2,
			clientCancelAfterNMsgs:          4,
			serverErrorAfterNMsgs:           -1,
			toSend:                          createTabularSensorData([]*structpb.Struct{msg6, msg7, msg8}),
			expSentAfterRetry:               createTabularSensorData([]*structpb.Struct{msg8}),
		},
		{
			name:                            `Binary upload should resume after server disconnection.`,
			dataType:                        v1.DataType_DATA_TYPE_BINARY_SENSOR,
			sendAckEveryNSensorDataMessages: 2,
			clientCancelAfterNMsgs:          -1,
			serverErrorAfterNMsgs:           4,
			toSend:                          createBinarySensorData([][]byte{msg1, msg2, msg3, msg4, msg5}),
			expSentAfterRetry:               createBinarySensorData([][]byte{msg3, msg4, msg5}),
		},
		{
			name:                            `Tabular upload should resume after server disconnection.`,
			dataType:                        v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			sendAckEveryNSensorDataMessages: 2,
			clientCancelAfterNMsgs:          -1,
			serverErrorAfterNMsgs:           4,
			toSend:                          createTabularSensorData([]*structpb.Struct{msg6, msg7, msg8}),
			expSentAfterRetry:               createTabularSensorData([]*structpb.Struct{msg8}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp data capture file.
			f, err := createTmpDataCaptureFile()
			test.That(t, err, test.ShouldBeNil)

			// First write metadata to file.
			captureMetadata := v1.DataCaptureMetadata{
				ComponentType:    componentType,
				ComponentName:    componentName,
				ComponentModel:   componentModel,
				MethodName:       methodName,
				Type:             tc.dataType,
				MethodParameters: nil,
				FileExtension:    datacapture.GetFileExt(tc.dataType, methodName, nil),
			}
			if _, err := pbutil.WriteDelimited(f, &captureMetadata); err != nil {
				t.Errorf("cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v", err)
			}

			// Next write sensor data to file.
			if err := writeSensorData(f, tc.toSend); err != nil {
				t.Errorf("%s: cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing: %v", tc.name, err)
			}

			// Progress file path which corresponds to the file which will be uploaded.
			// TODO: use function for getting progress file path in code and here
			progressFile := filepath.Join(viamProgressDotDir, filepath.Base(f.Name()))
			defer os.Remove(progressFile)

			// Stand up mock server. Register mock datasync service with a mock server.
			cancelChannel := make(chan bool)
			doneCancelChannel := make(chan bool)
			logger, _ := golog.NewObservedTestLogger(t)
			mockService := getMockService()
			mockService.clientContextCancelIndex = tc.clientCancelAfterNMsgs
			mockService.sendAckEveryNSensorDataMessages = tc.sendAckEveryNSensorDataMessages
			mockService.cancelChannel = cancelChannel
			mockService.doneCancelChannel = doneCancelChannel
			mockService.failAtIndex = tc.serverErrorAfterNMsgs

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
			go func() {
				<-cancelChannel
				time.Sleep(10 * time.Millisecond)
				sut.Close()
				doneCancelChannel <- true
			}()
			sut.Sync([]string{f.Name()})
			time.Sleep(syncWaitTime)

			// Validate client sent mockService the upload requests we would expect before canceling the upload.
			// TODO: refactor to not need this -1
			var expMsgs []*v1.UploadRequest
			var act []*v1.UploadRequest

			var retryIndex int
			switch {
			case tc.clientCancelAfterNMsgs != -1:
				retryIndex = tc.clientCancelAfterNMsgs
			case tc.serverErrorAfterNMsgs != -1:
				retryIndex = int(tc.serverErrorAfterNMsgs)
			default:
				retryIndex = len(tc.toSend)
			}
			expMsgs = buildSensorDataUploadRequests(tc.toSend[:retryIndex-1], tc.dataType, f.Name())
			act = mockService.getUploadRequests()[:retryIndex]

			compareUploadRequests(t, true, act, expMsgs)

			// For non-empty testcases, validate progress file & data capture file existences.
			if len(tc.toSend) > 0 {
				// Validate progress file exists for non-empty test cases.
				_, err = os.Stat(progressFile)
				test.That(t, err, test.ShouldBeNil)

				// Validate data capture file exists.
				_, err = os.Stat(f.Name())
				test.That(t, err, test.ShouldBeNil)
			}

			// Restart the server and register the service.
			mockService = getMockService()
			mockService.sendAckEveryNSensorDataMessages = tc.sendAckEveryNSensorDataMessages
			mockService.clientContextCancelIndex = -1
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

			// Validate client sent mockService the upload requests we would expect after resuming upload.
			expMsgs = buildSensorDataUploadRequests(tc.expSentAfterRetry, tc.dataType, f.Name())
			compareUploadRequests(t, true, mockService.getUploadRequests(), expMsgs)

			// Validate progress file does not exist.
			_, err = os.Stat(progressFile)
			test.That(t, err, test.ShouldNotBeNil)

			// Validate data capture file does not exist.
			_, err = os.Stat(f.Name())
			test.That(t, err, test.ShouldNotBeNil)
		})
	}
}

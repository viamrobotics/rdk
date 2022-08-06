package datasync

import (
	"context"
	"github.com/edaniels/golog"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
)

var testUploadFunc = func(ctx context.Context, client v1.DataSyncServiceClient, path string,
	partID string) error {
	return nil
}

// TODO: local server!!

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
		rpcServer, mockService := buildAndStartLocalServer(t, logger)
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
		sut, err := NewManager(logger, nil, partID, client, conn)
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
		time.Sleep(time.Millisecond * 100)

		sut.Close()
		// The mc.sent value should be the same as the expectedMsgs value.
		compareMetadata(t, mockService.getUploadRequests()[0].GetMetadata(), expectedMsgs[0].GetMetadata())
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
		rpcServer, mockService := buildAndStartLocalServer(t, logger)
		defer func() {
			err := rpcServer.Stop()
			test.That(t, err, test.ShouldBeNil)
		}()

		conn, err := getLocalServerConn(rpcServer, logger)
		test.That(t, err, test.ShouldBeNil)
		client := NewClient(conn)
		sut, err := NewManager(logger, nil, partID, client, conn)
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
		time.Sleep(100 * time.Millisecond)
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
		rpcServer, mockService := buildAndStartLocalServer(t, logger)
		defer func() {
			err := rpcServer.Stop()
			test.That(t, err, test.ShouldBeNil)
		}()

		conn, err := getLocalServerConn(rpcServer, logger)
		test.That(t, err, test.ShouldBeNil)
		client := NewClient(conn)
		sut, err := NewManager(logger, nil, partID, client, conn)
		test.That(t, err, test.ShouldBeNil)
		sut.Sync([]string{tf.Name()})

		// Create []v1.UploadRequest object from test case input 'expData []*structpb.Struct'.
		expectedMsgs := buildBinarySensorMsgs(tc.expData, filepath.Base(tf.Name()))

		// The mc.sent value should be the same as the expectedMsgs value.
		time.Sleep(100 * time.Millisecond)
		sut.Close()
		compareUploadRequests(t, true, mockService.getUploadRequests(), expectedMsgs)
	}
}

// Validates that for some captureDir, files are uploaded exactly once.
func TestUploadsOnce(t *testing.T) {
	// Register mock datasync service with a mock server.
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, mockService := buildAndStartLocalServer(t, logger)
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	conn, err := getLocalServerConn(rpcServer, logger)
	test.That(t, err, test.ShouldBeNil)
	client := NewClient(conn)
	sut, err := NewManager(logger, nil, partID, client, conn)
	test.That(t, err, test.ShouldBeNil)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile("", "whatever")
	file2, _ := ioutil.TempFile("", "whatever2")

	// Immediately try to Sync same files many times.
	for i := 1; i < 10; i++ {
		sut.Sync([]string{file1.Name(), file2.Name()})
	}

	// Verify upload was only called twice.
	time.Sleep(time.Millisecond * 100)
	sut.Close()
	test.That(t, mockService.getUploadCallCount(), test.ShouldEqual, 2)
	// TODO how to test different now?

	// Verify that the files were deleted after upload.
	_, err = os.Stat(file1.Name())
	test.That(t, err, test.ShouldNotBeNil)
	_, err = os.Stat(file2.Name())
	test.That(t, err, test.ShouldNotBeNil)
}

// TODO: how to test this?
func TestUploadExponentialRetry(t *testing.T) {
	// Set retry related global vars to faster values for test.
	initialWaitTime = time.Millisecond * 50
	maxRetryInterval = time.Millisecond * 150
	// Define an UploadFunc that fails 3 times then succeeds on its 4th attempt.
	// Register mock datasync service with a mock server.
	logger, _ := golog.NewObservedTestLogger(t)
	rpcServer, mockService := buildAndStartLocalServer(t, logger)
	mockService.failUntilIndex = 2
	defer func() {
		err := rpcServer.Stop()
		test.That(t, err, test.ShouldBeNil)
	}()

	conn, err := getLocalServerConn(rpcServer, logger)
	test.That(t, err, test.ShouldBeNil)
	client := NewClient(conn)
	sut, err := NewManager(logger, nil, partID, client, conn)
	test.That(t, err, test.ShouldBeNil)

	// Sync file.
	file1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(file1.Name())
	sut.Sync([]string{file1.Name()})

	// Let it run.
	time.Sleep(time.Second)
	sut.Close()

	test.That(t, mockService.getUploadCallCount(), test.ShouldEqual, int32(4))
}

func TestPartialUpload(t *testing.T) {
	msg1 := []byte("viam")
	msg2 := []byte("robotics")
	msg3 := []byte("builds cool software")
	msg4 := toProto(anyStruct{})
	msg5 := toProto(anyStruct{Field1: false})
	msg6 := toProto(anyStruct{Field1: true, Field2: 2020, Field3: "viam"})

	tests := []*partialUploadTestcase{
		{
			name: "Binary upload of non-empty file should resume from last point if it is " +
				"canceled.",
			toSend:                    createBinarySensorData([][]byte{msg1, msg2, msg3}),
			progressIndexWhenCanceled: 2,
			expDataBeforeCanceled:     createBinarySensorData([][]byte{msg1, msg2}),
			expDataAfterCanceled:      createBinarySensorData([][]byte{msg3}),
			dataType:                  v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Binary upload of empty file should not upload anything when it is started nor if it " +
				"is resumed.",
			toSend:                    createBinarySensorData([][]byte{}),
			progressIndexWhenCanceled: 0,
			expDataBeforeCanceled:     createBinarySensorData([][]byte{}),
			expDataAfterCanceled:      createBinarySensorData([][]byte{}),
			dataType:                  v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Binary upload with no more messages to send after it's canceled should not upload " +
				"anything after resuming.",
			toSend:                    createBinarySensorData([][]byte{msg1, msg2}),
			progressIndexWhenCanceled: 2,
			expDataBeforeCanceled:     createBinarySensorData([][]byte{msg1, msg2}),
			expDataAfterCanceled:      createBinarySensorData([][]byte{}),
			dataType:                  v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Binary upload that is interrupted before sending a single message should resume and send all" +
				"messages.",
			toSend:                    createBinarySensorData([][]byte{msg1, msg2}),
			progressIndexWhenCanceled: 0,
			expDataBeforeCanceled:     createBinarySensorData([][]byte{}),
			expDataAfterCanceled:      createBinarySensorData([][]byte{msg1, msg2}),
			dataType:                  v1.DataType_DATA_TYPE_BINARY_SENSOR,
		},
		{
			name: "Tabular upload of non-empty file should resume from last point if it is" +
				"canceled.",
			toSend:                    createTabularSensorData([]*structpb.Struct{msg4, msg5, msg6}),
			progressIndexWhenCanceled: 2,
			expDataBeforeCanceled:     createTabularSensorData([]*structpb.Struct{msg4, msg5}),
			expDataAfterCanceled:      createTabularSensorData([]*structpb.Struct{msg6}),
			dataType:                  v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		},
		{
			name: "Tabular upload of empty file should not upload anything when it is started nor if it " +
				"is resumed.",
			toSend:                    createTabularSensorData([]*structpb.Struct{}),
			progressIndexWhenCanceled: 0,
			expDataBeforeCanceled:     createTabularSensorData([]*structpb.Struct{}),
			expDataAfterCanceled:      createTabularSensorData([]*structpb.Struct{}),
			dataType:                  v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp data capture file.
			tf, err := createTmpDataCaptureFile()
			if err != nil {
				t.Errorf("%s cannot create temporary data capture file for testing: %v", tc.name, err)
			}
			defer os.Remove(tf.Name())

			writeCaptureMetadataToFile(t, tc.dataType, tf)

			// Next write sensor data to file.
			if err := writeSensorData(tf, tc.toSend); err != nil {
				t.Errorf("%s: cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing: %v", tc.name, err)
			}

			// Sync using mock client.
			mc := initMockClient(len(tc.expDataBeforeCanceled))
			sut := newTestSyncer(t, mc, nil)
			sut.Sync([]string{tf.Name()})
			time.Sleep(time.Millisecond * 100)
			compareUploadRequestsMockClient(t, false, mc, getUploadRequests(tc.expDataBeforeCanceled, tc.dataType,
				filepath.Base(tf.Name())))

			// Only verify progress file existence and content if the upload has expected messages after being canceled.
			path := filepath.Join(viamProgressDotDir, filepath.Base(tf.Name()))
			if len(tc.expDataAfterCanceled) > 0 {
				test.That(t, fileExists(path), test.ShouldBeTrue)
				progressIndex, err := sut.progressTracker.getProgressFileIndex(path)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, progressIndex, test.ShouldEqual, tc.progressIndexWhenCanceled)
			}
			sut.Close()

			// Reset mock client to be empty (simulates a full reboot of client). Then sync using mock client.
			mc = initMockClient(len(tc.expDataAfterCanceled))
			sut = newTestSyncer(t, mc, nil)
			sut.Sync([]string{tf.Name()})
			time.Sleep(time.Millisecond * 100)
			sut.Close()
			compareUploadRequestsMockClient(t, false, mc, getUploadRequests(tc.expDataAfterCanceled, tc.dataType,
				filepath.Base(tf.Name())))
			test.That(t, fileExists(path), test.ShouldBeFalse)
		})
	}
}

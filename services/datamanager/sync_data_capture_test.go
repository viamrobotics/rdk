package datamanager

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
)

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
		mc := &mockClient{
			sent:        []*v1.UploadRequest{},
			lock:        sync.Mutex{},
			cancelIndex: -1,
		}

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
		{
			name: `stream of a lot of binary sensor data readings should send multiple ACKs of data persisted to 
			GCS`,
			toSend:  [][]byte{msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3},
			expData: [][]byte{msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mc := &mockClient{
				sent:             []*v1.UploadRequest{},
				cancelIndex:      -1,
				sentSinceLastAck: 0,
				sendAckInterval:  2,
				sendAck:          false,
				sendEOF:          false,
				lock:             sync.Mutex{},
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
			expectedMsgs := buildBinarySensorMsgs(tc.expData, filepath.Base(tf.Name()))

			// The mc.sent value should be the same as the expectedMsgs value.
			time.Sleep(100 * time.Millisecond)
			sut.Close()
			compareUploadRequests(t, true, mc.sent, expectedMsgs)
		})
	}
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
			path := filepath.Join(progressDir, filepath.Base(tf.Name()))
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

type mockDataSyncServiceServer struct {
	messagesSent               int
	sendAckEveryNMessages      int
	cancelStreamAfterNMessages int
	shouldSendEOF              bool
	shouldSendACK              bool
	shouldSendCancelCtx        bool
	v1.UnimplementedDataSyncServiceServer
}

func (m mockDataSyncServiceServer) Upload(stream v1.DataSyncService_UploadServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		var retUploadResponse *v1.UploadResponse
		var retErr error
		if m.shouldSendACK {
			m.shouldSendACK = false
			retUploadResponse, retErr = &v1.UploadResponse{RequestsWritten: int32(m.messagesSent)}, nil
		}
		if m.shouldSendEOF {
			m.shouldSendEOF = false
			retUploadResponse, retErr = nil, io.EOF
		}
		if m.shouldSendCancelCtx {
			m.shouldSendCancelCtx = false
			retUploadResponse, retErr = nil, context.Canceled
		}
		if retErr != nil {
			return retErr
		}
		stream.Send(retUploadResponse)
		m.messagesSent++
	}
	return nil
}

type mockServerBehavior struct {
	sendAckEveryNMessages      int
	cancelStreamAfterNMessages int
	shouldSendEOF              bool
	shouldSendAck              bool
	shouldSendCancelCtx        bool
}

func TestDataCaptureUpload(t *testing.T) {
	msgBin1 := []byte("Robots are really cool.")
	msgBin2 := []byte("This work is helping develop the robotics space.")
	msgBin3 := []byte("This message is used for testing.")
	tests := []struct {
		name    string
		toSend  [][]byte
		expData [][]byte
		msb     *mockServerBehavior
	}{
		{
			name:    "stream of binary sensor data readings",
			toSend:  [][]byte{msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3},
			expData: [][]byte{msgBin1, msgBin2, msgBin3, msgBin1, msgBin2, msgBin3},
			msb: &mockServerBehavior{
				sendAckEveryNMessages:      2,
				cancelStreamAfterNMessages: -1,
				shouldSendEOF:              false,
				shouldSendAck:              false,
				shouldSendCancelCtx:        false,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mc := &mockDataSyncServiceServer{
				messagesSent:               0,
				sendAckEveryNMessages:      tc.msb.sendAckEveryNMessages,
				cancelStreamAfterNMessages: tc.msb.cancelStreamAfterNMessages,
				shouldSendEOF:              tc.msb.shouldSendEOF,
				shouldSendACK:              tc.msb.shouldSendAck,
				shouldSendCancelCtx:        tc.msb.shouldSendCancelCtx,
			}

			// Register mock datamanager service with a mock server.
			logger, _ := golog.NewObservedTestLogger(t)
			rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
			test.That(t, err, test.ShouldBeNil)
			rpcServer.RegisterServiceServer(
				context.Background(),
				&v1.DataSyncService_ServiceDesc,
				mc,
				v1.RegisterDataSyncServiceHandlerFromEndpoint,
			)

			// Stand up the server. Defer stopping the server.
			go func() {
				err := rpcServer.Start()
				test.That(t, err, test.ShouldBeNil)
			}()
			defer func() {
				err := rpcServer.Stop()
				test.That(t, err, test.ShouldBeNil)
			}()

			conn, err := rpc.DialDirectGRPC(
				context.Background(),
				rpcServer.InternalAddr().String(),
				logger,
				rpc.WithInsecure(),
			)
			test.That(t, err, test.ShouldBeNil)

			// Defer closing the connection.
			defer func() {
				err := conn.Close()
				test.That(t, err, test.ShouldBeNil)
			}()

			// Create temp file to be used as examples of reading data from the files into buffers and finally to have
			// that data be uploaded to the cloud.
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

			// Create the DataSyncService_UploadClient to send a stream of requests to the server and will receive a
			// stream of responses from the server.
			client := v1.NewDataSyncServiceClient(conn)
			uploadClient, err := client.Upload(context.Background())
			test.That(t, err, test.ShouldBeNil)

			// Create and initialize the syncer to begin upload process.
			sut := newTestSyncerRealClient(t, uploadClient, nil)
			sut.Sync([]string{tf.Name()})
			sut.Close()

		})
	}

}

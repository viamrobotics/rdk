package datasync

import (
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/edaniels/golog"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/test"
)

const (
	syncWaitTime = 500 * time.Millisecond
)

// TODO: update tests

// TODO: add back when arbitrary file upload is re-added
//func TestFileUpload(t *testing.T) {
//	uploadChunkSize = 10
//	msgContents := []byte("This is part of testing in datamanager service in RDK.")
//
//	tests := []struct {
//		name    string
//		toSend  []byte
//		expData [][]byte
//	}{
//		{
//			name:    "Empty file should not send only metadata.",
//			toSend:  []byte(""),
//			expData: [][]byte{},
//		},
//		{
//			name:   "Non empty file should successfully send its content in chunks.",
//			toSend: msgContents,
//			expData: [][]byte{
//				msgContents[:10], msgContents[10:20], msgContents[20:30], msgContents[30:40],
//				msgContents[40:50], msgContents[50:],
//			},
//		},
//	}
//
//	for _, tc := range tests {
//		t.Run(tc.name, func(t *testing.T) {
//			// Register mock datasync service with a mock server.
//			logger, _ := golog.NewObservedTestLogger(t)
//			mockService := getMockService()
//			rpcServer := buildAndStartLocalServer(t, logger, mockService)
//			defer func() {
//				err := rpcServer.Stop()
//				test.That(t, err, test.ShouldBeNil)
//			}()
//
//			// Create temp file to be used as examples of reading data from the files into buffers
//			// (and finally to have that data be uploaded) to the cloud.
//			tf, err := ioutil.TempFile("", "")
//			test.That(t, err, test.ShouldBeNil)
//			defer os.Remove(tf.Name())
//
//			// Write the data from test cases into the temp file to prepare for reading by the fileUpload function.
//			_, err = tf.Write(tc.toSend)
//			test.That(t, err, test.ShouldBeNil)
//
//			conn, err := getLocalServerConn(rpcServer, logger)
//			test.That(t, err, test.ShouldBeNil)
//			client := NewClient(conn)
//			sut, err := NewManager(logger, partID, client, conn)
//			test.That(t, err, test.ShouldBeNil)
//			sut.Sync([]string{tf.Name()})
//			time.Sleep(syncWaitTime)
//			sut.Close()
//
//			// Validate the expected messages were sent.
//			expectedMsgs := buildFileDataUploadRequests(tc.expData, filepath.Base(tf.Name()))
//			test.That(t, len(mockService.getUploadRequests()), test.ShouldEqual, len(expectedMsgs))
//			if len(expectedMsgs) > 1 {
//				test.That(t, mockService.getUploadRequests()[0].GetMetadata().String(), test.ShouldResemble,
//					expectedMsgs[0].GetMetadata().String())
//				for i := range expectedMsgs[1:] {
//					test.That(t, string(mockService.getUploadRequests()[i].GetFileContents().GetData()),
//						test.ShouldEqual, string(expectedMsgs[i].GetFileContents().GetData()))
//				}
//			}
//		})
//	}
//}

func TestSensorUploadTabular(t *testing.T) {
	protoMsgTabularStruct := toProto(anyStruct{})

	tests := []struct {
		name   string
		toSend *v1.SensorData
		count  int
	}{
		{
			name: "One sensor data.",
			toSend: &v1.SensorData{
				Data: &v1.SensorData_Struct{
					Struct: protoMsgTabularStruct,
				},
			},
			count: 1,
		},
		{
			name: "Many sensor data.",
			toSend: &v1.SensorData{
				Data: &v1.SensorData_Struct{
					Struct: protoMsgTabularStruct,
				},
			},
			count: 1000,
		},
	}

	for _, tc := range tests {
		t.Log(tc.name)
		tmpDir, err := ioutil.TempDir("", "")
		test.That(t, err, test.ShouldBeNil)

		// Create temp data capture file.
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
		q := datacapture.NewDeque(tmpDir, &captureMetadata)

		// Write the data from the test cases into the files to prepare them for reading by the fileUpload function
		for i := 0; i < tc.count; i++ {
			err := q.Enqueue(tc.toSend)
			test.That(t, err, test.ShouldBeNil)
		}
		err = q.Sync()
		q.Close()
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
		sut.Sync([]*datacapture.Deque{q})
		time.Sleep(syncWaitTime)
		sut.Close()

		// Validate the client sent the expected messages.
		act := mockService.getUnaryUploadRequests()
		expMetadata := &v1.UploadMetadata{
			PartId:           partID,
			ComponentType:    captureMetadata.GetComponentType(),
			ComponentName:    captureMetadata.GetComponentName(),
			ComponentModel:   captureMetadata.GetComponentModel(),
			MethodName:       captureMetadata.GetMethodName(),
			Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
			MethodParameters: captureMetadata.GetMethodParameters(),
			Tags:             tags,
		}

		// Validate that all readings were uploaded.
		written := 0
		for _, ur := range act {
			test.That(t, ur.GetMetadata().String(), test.ShouldEqual, expMetadata.String())
			for _, content := range ur.GetSensorContents() {
				test.That(t, content.GetStruct().String(), test.ShouldResemble, tc.toSend.GetStruct().String())
				written += 1
			}
		}
		test.That(t, written, test.ShouldEqual, tc.count)

		// Validate files were deleted after syncing.
		files := getAllFiles(tmpDir)
		test.That(t, len(files), test.ShouldEqual, 0)
	}
}

func TestSensorUploadBinary(t *testing.T) {
	msgContents := []byte("This is a message. This message is part of testing in datamanager service in RDK.")

	tests := []struct {
		name   string
		toSend *v1.SensorData
		count  int
	}{
		{
			name: "One binary data.",
			toSend: &v1.SensorData{
				Data: &v1.SensorData_Binary{
					Binary: msgContents,
				},
			},
			count: 1,
		},
		{
			name: "Many binary data.",
			toSend: &v1.SensorData{
				Data: &v1.SensorData_Binary{
					Binary: msgContents,
				},
			},
			count: 20,
		},
	}

	for _, tc := range tests {
		t.Log(tc.name)
		tmpDir, err := ioutil.TempDir("", "")
		test.That(t, err, test.ShouldBeNil)

		// Create temp data capture file.
		captureMetadata := v1.DataCaptureMetadata{
			ComponentType:    componentType,
			ComponentName:    componentName,
			ComponentModel:   componentModel,
			MethodName:       methodName,
			Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			MethodParameters: nil,
			FileExtension:    datacapture.GetFileExt(v1.DataType_DATA_TYPE_TABULAR_SENSOR, methodName, nil),
			Tags:             tags,
		}
		q := datacapture.NewDeque(tmpDir, &captureMetadata)

		// Write the data from the test cases into the files to prepare them for reading by the fileUpload function
		for i := 0; i < tc.count; i++ {
			err := q.Enqueue(tc.toSend)
			test.That(t, err, test.ShouldBeNil)
		}
		err = q.Sync()
		q.Close()
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
		sut.Sync([]*datacapture.Deque{q})
		time.Sleep(syncWaitTime)
		sut.Close()

		// Validate the client sent the expected messages.
		act := mockService.getUnaryUploadRequests()
		expMetadata := &v1.UploadMetadata{
			PartId:           partID,
			ComponentType:    captureMetadata.GetComponentType(),
			ComponentName:    captureMetadata.GetComponentName(),
			ComponentModel:   captureMetadata.GetComponentModel(),
			MethodName:       captureMetadata.GetMethodName(),
			Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			MethodParameters: captureMetadata.GetMethodParameters(),
			Tags:             tags,
		}

		// Validate that all readings were uploaded.
		written := 0
		for _, ur := range act {
			test.That(t, ur.GetMetadata().String(), test.ShouldEqual, expMetadata.String())
			for _, content := range ur.GetSensorContents() {
				test.That(t, string(content.GetBinary()), test.ShouldResemble, string(tc.toSend.GetBinary()))
				written += 1
			}
		}
		test.That(t, written, test.ShouldEqual, tc.count)

		// Validate files were deleted after syncing.
		files := getAllFiles(tmpDir)
		test.That(t, len(files), test.ShouldEqual, 0)
	}
}

// Validates that for some captureDir, files are uploaded exactly once.
func TestUploadsOnce(t *testing.T) {
	protoMsgTabularStruct := toProto(anyStruct{})
	sd := &v1.SensorData{
		Data: &v1.SensorData_Struct{
			Struct: protoMsgTabularStruct,
		},
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

	// Put a couple files in captureDir.
	tmpDir, err := ioutil.TempDir("", "")
	test.That(t, err, test.ShouldBeNil)
	q := datacapture.NewDeque(tmpDir, &captureMetadata)

	// Write the data from the test cases into the files to prepare them for reading by the fileUpload function
	for i := 0; i < 100; i++ {
		err := q.Enqueue(sd)
		test.That(t, err, test.ShouldBeNil)
	}
	err = q.Sync()
	q.Close()
	test.That(t, err, test.ShouldBeNil)

	// Immediately try to Sync same files many times.
	for i := 1; i < 5; i++ {
		sut.Sync([]*datacapture.Deque{q})
	}
	time.Sleep(syncWaitTime)
	sut.Close()

	act := mockService.getUnaryUploadRequests()
	expMetadata := &v1.UploadMetadata{
		PartId:           partID,
		ComponentType:    captureMetadata.GetComponentType(),
		ComponentName:    captureMetadata.GetComponentName(),
		ComponentModel:   captureMetadata.GetComponentModel(),
		MethodName:       captureMetadata.GetMethodName(),
		Type:             v1.DataType_DATA_TYPE_TABULAR_SENSOR,
		MethodParameters: captureMetadata.GetMethodParameters(),
		Tags:             tags,
	}

	written := 0
	for _, ur := range act {
		test.That(t, ur.GetMetadata().String(), test.ShouldResemble, expMetadata.String())
		for _, content := range ur.GetSensorContents() {
			test.That(t, content.GetStruct().String(), test.ShouldResemble, sd.GetStruct().String())
			written += 1
		}
	}
	test.That(t, written, test.ShouldEqual, 100)

	// Validate files were deleted after syncing.
	files := getAllFiles(tmpDir)
	test.That(t, len(files), test.ShouldEqual, 0)
}

func TestUploadExponentialRetry(t *testing.T) {
	// Set retry related global vars to faster values for test.
	initialWaitTimeMillis.Store(50)
	maxRetryInterval = time.Millisecond * 150
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
			protoMsgTabularStruct := toProto(anyStruct{})
			sd := &v1.SensorData{
				Data: &v1.SensorData_Struct{
					Struct: protoMsgTabularStruct,
				},
			}
			tmpDir, err := ioutil.TempDir("", "")
			test.That(t, err, test.ShouldBeNil)

			// Create temp data capture file.
			captureMetadata := v1.DataCaptureMetadata{
				ComponentType:    componentType,
				ComponentName:    componentName,
				ComponentModel:   componentModel,
				MethodName:       methodName,
				Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
				MethodParameters: nil,
				FileExtension:    datacapture.GetFileExt(v1.DataType_DATA_TYPE_TABULAR_SENSOR, methodName, nil),
				Tags:             tags,
			}
			q := datacapture.NewDeque(tmpDir, &captureMetadata)

			// Write the data from the test cases into the files to prepare them for reading by the fileUpload function
			for i := 0; i < 5; i++ {
				err := q.Enqueue(sd)
				test.That(t, err, test.ShouldBeNil)
			}
			q.Close()
			err = q.Sync()
			test.That(t, err, test.ShouldBeNil)

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
			sut.Sync([]*datacapture.Deque{q})

			// Let it run so it can retry (or not).
			time.Sleep(tc.waitTime)
			sut.Close()

			// Validate that the client called Upload the correct number of times, and whether or not the file was
			// deleted.
			test.That(t, mockService.callCount.Load(), test.ShouldEqual, tc.expCallCount)
			// Validate files were deleted after syncing.
			files := getAllFiles(tmpDir)
			if tc.shouldStillExist {
				test.That(t, len(files), test.ShouldNotEqual, 0)
			} else {
				test.That(t, len(files), test.ShouldEqual, 0)
			}
		})
	}
}

func getAllFiles(dir string) []string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files
}

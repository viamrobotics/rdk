package datamanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

var (
	messageShutdownLimit = 2
)

type mockClientWithShutdown struct {
	sent []*v1.UploadRequest
	lock sync.Mutex
	grpc.ClientStream
}

func (m *mockClientWithShutdown) Send(req *v1.UploadRequest) error {
	m.lock.Lock()
	if len(m.sent) < (messageShutdownLimit + 1) {
		m.sent = append(m.sent, req)
		m.lock.Unlock()
		return nil
	}
	m.lock.Unlock()
	return errors.New("simulate client shutdown")
}

func (m *mockClientWithShutdown) CloseAndRecv() (*v1.UploadResponse, error) {
	return &v1.UploadResponse{}, nil
}

func (m *mockClientWithShutdown) Context() context.Context {
	return context.TODO()
}

// Builds syncer used in partial upload tests.
func newTestSyncerPartialUploads(t *testing.T, mc *mockClientWithShutdown, uploadFn uploadFn) *syncer {
	t.Helper()
	l := golog.NewTestLogger(t)
	ret := *newSyncer(l, uploadFn, partID)
	ret.client = mc
	return &ret
}

type progress struct {
	nextSensorReadingIndex int
}

func TestPartialUpload(t *testing.T) {
	msg1 := []byte("viam")
	msg2 := []byte("robotics")
	msg3 := []byte("builds cool software")
	progressTC1Breakpoint := &progress{
		nextSensorReadingIndex: 2,
	}
	tests := []struct {
		name                  string
		toSend                [][]byte
		progressAtBreakpoint  progress
		expDataBeforeShutdown [][]byte
		expDataAfterShutdown  [][]byte
	}{
		{
			name:                  "not empty",
			toSend:                [][]byte{msg1, msg2, msg3},
			progressAtBreakpoint:  *progressTC1Breakpoint,
			expDataBeforeShutdown: [][]byte{msg1, msg2},
			expDataAfterShutdown:  [][]byte{msg3},
		},
	}

	for _, tc := range tests {

		mc := &mockClientWithShutdown{
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
		captureMetadata := v1.DataCaptureMetadata{
			ComponentType:    componentType,
			ComponentName:    componentName,
			MethodName:       methodName,
			Type:             v1.DataType_DATA_TYPE_BINARY_SENSOR,
			MethodParameters: nil,
		}
		if _, err := pbutil.WriteDelimited(tf, &captureMetadata); err != nil {
			t.Errorf("%s cannot write protobuf struct to temporary file as part of setup for sensorUpload testing: %v",
				tc.name, err)
		}

		expectedMsgsBeforeShutdown := buildExpMsgs(tc.expDataBeforeShutdown, tf.Name())
		expectedMsgsAfterShutdown := buildExpMsgs(tc.expDataAfterShutdown, tf.Name())

		if err := writeBinarySensorData(tf, tc.toSend); err != nil {
			t.Errorf("%s: cannot write byte slice to temporary file as part of setup for sensorUpload/fileUpload testing: %v", tc.name, err)
		}
		time.Sleep(time.Millisecond * 100)

		sut := newTestSyncerPartialUploads(t, mc, nil)
		sut.Sync([]string{tf.Name()})
		time.Sleep(time.Millisecond * 100)
		sut.Close()

		path := filepath.Join("progress_dir", filepath.Base(tf.Name()))

		compareUploadRequests(t, false, mc.sent, expectedMsgsBeforeShutdown)
		verifyFileExistence(t, path, true)
		verifyProgressFile(t, tc.progressAtBreakpoint, path)

		sut = newTestSyncerPartialUploads(t, mc, nil)
		sut.Sync([]string{tf.Name()})
		time.Sleep(time.Millisecond * 100)
		sut.Close()

		compareUploadRequests(t, false, mc.sent, expectedMsgsAfterShutdown)
		verifyFileExistence(t, path, false)

	}

}

// NEED TO IMPLEMENT ONCE WE'VE DECIDED ON AN FILE TYPE & ORGANIZATION FOR PROGRESS FILE
func getProgressFromProgressFile(progressFileName string) (*progress, error) {
	bs, err := ioutil.ReadFile(progressFileName)
	if err != nil {
		return nil, err
	}
	i, err := bytesToInt(bs)
	if err != nil {
		return nil, err
	}
	return &progress{nextSensorReadingIndex: i}, nil
}

// WAITING ON `getProgressFromProgressFile`
func verifyProgressFile(t *testing.T, progressAtBreakpoint progress, progressFileName string) {
	t.Helper()
	progress, _ := getProgressFromProgressFile(progressFileName)
	printProgress(progressAtBreakpoint, false)
	printProgress(*progress, true)
	test.That(t, reflect.DeepEqual(progressAtBreakpoint, *progress), test.ShouldBeTrue)
}

func printProgress(p progress, isActual bool) {
	if isActual {
		fmt.Println("\n...Actual value...")
	} else {
		fmt.Println("\n...Expected value...")
	}
	fmt.Println("next sensor reading index: ", p.nextSensorReadingIndex)
}

func verifyFileExistence(t *testing.T, fileName string, shouldExist bool) {
	t.Helper()
	_, err := os.Stat(fileName)
	test.That(t, errors.Is(err, os.ErrNotExist), test.ShouldNotEqual, shouldExist)
}

// build expected messages from test case input that begins with proper metadata message
func buildExpMsgs(expData [][]byte, fileName string) []*v1.UploadRequest {
	var expMsgs []*v1.UploadRequest
	expMsgs = append(expMsgs, &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: &v1.UploadMetadata{
				PartId:   partID,
				Type:     v1.DataType_DATA_TYPE_BINARY_SENSOR,
				FileName: fileName,
			},
		},
	})
	for _, expMsg := range expData {
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

package datamanager

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/edaniels/golog"
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
	println("m.sent (actual) length", len(m.sent))
	if len(m.sent) < messageShutdownLimit {
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
	fileName               string
	nextSensorReadingIndex int
}

func TestPartialUpload(t *testing.T) {
	msg1 := []byte("viam")
	msg2 := []byte("robotics")
	msg3 := []byte("builds cool software")
	progressTC1Breakpoint := &progress{
		fileName:               "",
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

		tf, err := ioutil.TempFile("", "*.capture")
		if err != nil {
			t.Errorf("%s: cannot create temporary file to be used for sensorUpload/fileUpload testing: %v", tc.name, err)
		}
		defer os.Remove(tf.Name())

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

		compareUploadRequests(t, false, mc.sent, expectedMsgsBeforeShutdown)
		verifyFileExistence(t, tf.Name()+"_progress", true)
		verifyProgressFile(t, tc.progressAtBreakpoint, tf.Name()+"_progress")

		sut = newTestSyncerPartialUploads(t, mc, nil)
		sut.Sync([]string{tf.Name()})
		time.Sleep(time.Millisecond * 100)
		sut.Close()

		compareUploadRequests(t, false, mc.sent, expectedMsgsAfterShutdown)
		verifyFileExistence(t, tf.Name()+"_progress", false)

	}

}

// NEED TO IMPLEMENT ONCE WE'VE DECIDED ON AN FILE TYPE & ORGANIZATION FOR PROGRESS FILE
func getProgressFromProgressFile(progressFileName string) (*progress, error) {
	f, err := os.Open(progressFileName)
	if err != nil {
		return nil, err
	}
	return &progress{fileName: f.Name(), nextSensorReadingIndex: 0}, nil
}

// WAITING ON `getProgressFromProgressFile`
func verifyProgressFile(t *testing.T, progressAtBreakpoint progress, progressFileName string) {
	t.Helper()
	progress, _ := getProgressFromProgressFile(progressFileName)
	test.That(t, reflect.DeepEqual(progressAtBreakpoint, progress), test.ShouldEqual)
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

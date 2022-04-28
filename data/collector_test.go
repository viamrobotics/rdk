package data

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"go.viam.com/test"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "go.viam.com/rdk/proto/api/service/datamanager/v1"
)

type exampleReading struct {
	Field1 bool
}

func (r *exampleReading) toProto() *structpb.Struct {
	msg, err := StructToStructPb(r)
	if err != nil {
		return nil
	}
	return msg
}

var (
	dummyCapturer = CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		time.Sleep(time.Millisecond * 5)
		return dummyReading, nil
	})
	dummyReading      = exampleReading{}
	dummyReadingProto = dummyReading.toProto()
	queueSize         = 250
	bufferSize        = 4096
)

func TestNewCollector(t *testing.T) {
	c := NewCollector(nil, time.Second, nil, nil, queueSize, bufferSize, nil)
	test.That(t, c, test.ShouldNotBeNil)
}

// Test that SensorData is written correctly and can be read, and that interval is respected and that capture()
// is called floor(time_passed/interval) times.
func TestSuccessfulWrite(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	c := NewCollector(
		dummyCapturer, time.Millisecond*25, map[string]string{"name": "test"}, target1, queueSize, bufferSize, l)
	go c.Collect()

	// Verify that it writes to the file at all.
	time.Sleep(time.Millisecond * 70)
	c.Close()
	fileSize := getFileSize(target1)
	test.That(t, fileSize, test.ShouldBeGreaterThan, 0)

	// Verify that the data it wrote matches what we expect (two SensorData's containing dummyReading).
	_, _ = target1.Seek(0, 0)
	// Give 20ms of leeway so slight changes in execution ordering don't impact the test.
	// floor(70/25) = 2
	for i := 0; i < 2; i++ {
		read, err := readNextSensorData(target1)
		if err != nil {
			t.Fatalf("failed to read SensorData from file: %v", err)
		}
		test.That(t, proto.Equal(dummyReadingProto, read.Data), test.ShouldBeTrue)
	}

	// Next reading should fail; there should only be two readings.
	_, err := readNextSensorData(target1)
	test.That(t, err, test.ShouldEqual, io.EOF)
}

func TestClose(t *testing.T) {
	// Set up a collector.
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	c := NewCollector(
		dummyCapturer, time.Millisecond*15, map[string]string{"name": "test"}, target1, queueSize, bufferSize, l)
	go c.Collect()
	time.Sleep(time.Millisecond * 25)

	// Close and measure fileSize.
	c.Close()
	fileSize := getFileSize(target1)

	// Assert capture is no longer being called/file is no longer being written to.
	time.Sleep(time.Millisecond * 25)
	test.That(t, getFileSize(target1), test.ShouldEqual, fileSize)
}

func TestSetTarget(t *testing.T) {
	l := golog.NewTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever1")
	target2, _ := ioutil.TempFile("", "whatever2")
	defer os.Remove(target1.Name())
	defer os.Remove(target2.Name())

	c := NewCollector(
		dummyCapturer, time.Millisecond*15, map[string]string{"name": "test"}, target1, queueSize, bufferSize, l)
	go c.Collect()
	time.Sleep(time.Millisecond * 30)

	// Change target, verify that target1 was written to.
	c.SetTarget(target2)
	sizeTgt1 := getFileSize(target1)
	test.That(t, getFileSize(target1), test.ShouldBeGreaterThan, 0)

	// Verify that tgt2 was written to, and that target1 was not written to after the target was changed.
	time.Sleep(time.Millisecond * 30)
	c.Close()
	test.That(t, getFileSize(target1), test.ShouldEqual, sizeTgt1)
	test.That(t, getFileSize(target2), test.ShouldBeGreaterThan, 0)
}

// Verifies that Collect does not error if it receives a single error when calling capture, and that those errors are
// logged.
func TestSwallowsErrors(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())

	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		return nil, errors.New("error")
	})
	c := NewCollector(
		errorCapturer, time.Millisecond*10, map[string]string{"name": "test"}, target1, queueSize, bufferSize, logger)
	errorChannel := make(chan error)
	defer close(errorChannel)
	go func() {
		err := c.Collect()
		if err != nil {
			errorChannel <- err
		}
	}()
	time.Sleep(30 * time.Millisecond)
	c.Close()

	// Verify that no errors were passed into errorChannel, and that errors were logged.
	select {
	case err := <-errorChannel:
		logger.Fatalf("Collector.Collect propogated error: %s", err)
	default:
		test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldBeGreaterThan, 0)
	}
}

// TestCtxCancelledLoggedAsDebug verifies that context cancelled errors are logged as debug level instead of as errors.
func TestCtxCancelledLoggedAsDebug(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)
	target1, _ := ioutil.TempFile("", "whatever")
	defer os.Remove(target1.Name())
	errorCapturer := CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		return nil, fmt.Errorf("arbitrary wrapping message: %w", context.Canceled)
	})
	c := NewCollector(
		errorCapturer, time.Millisecond*10, map[string]string{"name": "test"}, target1, queueSize, bufferSize, logger)
	go c.Collect()
	time.Sleep(30 * time.Millisecond)
	c.Close()

	test.That(t, logs.FilterLevelExact(zapcore.DebugLevel).Len(), test.ShouldBeGreaterThan, 0)
	test.That(t, logs.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 0)
}

func getFileSize(f *os.File) int64 {
	fileInfo, err := f.Stat()
	if err != nil {
		return 0
	}
	return fileInfo.Size()
}

func readNextSensorData(f *os.File) (*v1.SensorData, error) {
	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, err
	}
	return r, nil
}
